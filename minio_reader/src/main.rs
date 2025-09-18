//! 最小化 MinIO(S3兼容)对象下载工具
//! 仅依赖 reqwest，直接实现 S3 V4 签名，包体积极小

use std::env;
use std::process::exit;
use std::io::{self, Write};
use clap::Parser;
use dotenvy::dotenv;
use chrono::{Utc, DateTime};
use hmac::{Hmac, Mac};
use sha2::{Sha256, Digest};
use reqwest::Client;
use reqwest::header::{HeaderMap, HeaderValue, HOST, CONTENT_TYPE, AUTHORIZATION};
use tokio::io::AsyncWriteExt;

/// 命令行参数结构体
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// MinIO对象路径，格式为 /bucket/object/path
    object_path: String,
    /// 读取大小，单位KB，留空则读取完整文件
    transfer_size: Option<usize>,
}

/// 计算SHA256十六进制字符串
fn sha256_hex(data: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(data);
    format!("{:x}", hasher.finalize())
}

/// 生成 AWS S3 V4 签名（仅支持 GET Object，最小实现）
fn s3_sign_v4(
    method: &str,
    uri: &str,
    host: &str,
    region: &str,
    access_key: &str,
    secret_key: &str,
    query: &str,
    payload_hash: &str,
    now: DateTime<Utc>,
) -> (String, String) {
    let amz_date = now.format("%Y%m%dT%H%M%SZ").to_string();
    let date = now.format("%Y%m%d").to_string();
    let credential_scope = format!("{}/{}/s3/aws4_request", date, region);

    // Canonical Request
    let canonical_headers = format!("host:{}\nx-amz-content-sha256:{}\nx-amz-date:{}\n", host, payload_hash, amz_date);
    let signed_headers = "host;x-amz-content-sha256;x-amz-date";
    let canonical_request = format!(
        "{method}\n{uri}\n{query}\n{canonical_headers}\n{signed_headers}\n{payload_hash}",
        method=method,
        uri=uri,
        query=query,
        canonical_headers=canonical_headers,
        signed_headers=signed_headers,
        payload_hash=payload_hash
    );
    let canonical_request_hash = sha256_hex(canonical_request.as_bytes());

    // String to Sign
    let string_to_sign = format!(
        "AWS4-HMAC-SHA256\n{amz_date}\n{credential_scope}\n{canonical_request_hash}",
        amz_date=amz_date,
        credential_scope=credential_scope,
        canonical_request_hash=canonical_request_hash
    );

    // Signature Key
    let k_date = hmac_sha256(format!("AWS4{}", secret_key).as_bytes(), date.as_bytes());
    let k_region = hmac_sha256(&k_date, region.as_bytes());
    let k_service = hmac_sha256(&k_region, b"s3");
    let k_signing = hmac_sha256(&k_service, b"aws4_request");
    let signature = hmac_sha256_hex(&k_signing, string_to_sign.as_bytes());

    // Authorization header
    let auth = format!(
        "AWS4-HMAC-SHA256 Credential={}/{}, SignedHeaders={}, Signature={}",
        access_key, credential_scope, signed_headers, signature
    );
    (auth, amz_date)
}

/// HMAC-SHA256
fn hmac_sha256(key: &[u8], data: &[u8]) -> Vec<u8> {
    let mut mac = Hmac::<Sha256>::new_from_slice(key).unwrap();
    mac.update(data);
    mac.finalize().into_bytes().to_vec()
}
fn hmac_sha256_hex(key: &[u8], data: &[u8]) -> String {
    let mut mac = Hmac::<Sha256>::new_from_slice(key).unwrap();
    mac.update(data);
    format!("{:x}", mac.finalize().into_bytes())
}

#[tokio::main]
async fn main() {
    dotenv().ok(); // 加载.env（如有）

    // 读取环境变量
    let endpoint = env::var("DATAFLUX_OSS_ADDRESS").unwrap_or_else(|_| {
        eprintln!("环境变量 DATAFLUX_OSS_ADDRESS 未设置");
        exit(1);
    });
    let access_key = env::var("DATAFLUX_OSS_USERNAME").unwrap_or_else(|_| {
        eprintln!("环境变量 DATAFLUX_OSS_USERNAME 未设置");
        exit(1);
    });
    let secret_key = env::var("DATAFLUX_OSS_PASSWORD").unwrap_or_else(|_| {
        eprintln!("环境变量 DATAFLUX_OSS_PASSWORD 未设置");
        exit(1);
    });

    // 解析命令行参数
    let args = Args::parse();
    let object_path = args.object_path.trim_start_matches('/'); // 去除前导斜杠
    let mut parts = object_path.splitn(2, '/');
    let bucket = parts.next().unwrap_or("");
    let key = parts.next().unwrap_or("");
    if bucket.is_empty() || key.is_empty() {
        eprintln!("objectPath 格式错误，需为 /bucket/object/path");
        exit(1);
    }

    // 读取 transfer_size
    let transfer_size_bytes = args.transfer_size.map(|kb| kb * 1024);

    // 构造 S3 GET Object URL
    let region = "us-east-1"; // MinIO 默认兼容
    let url = format!("{}/{}/{}", endpoint.trim_end_matches('/'), bucket, key);

    // S3 V4 签名
    let now = Utc::now();
    let uri = format!("/{}/{}", bucket, key);
    let host = endpoint.trim_start_matches("http://").trim_start_matches("https://");
    let query = "";
    let payload_hash = sha256_hex(b"");
    let (auth, amz_date) = s3_sign_v4(
        "GET",
        &uri,
        host,
        region,
        &access_key,
        &secret_key,
        query,
        &payload_hash,
        now,
    );

    // 构造请求头
    let mut headers = HeaderMap::new();
    headers.insert(HOST, HeaderValue::from_str(host).unwrap());
    headers.insert("x-amz-content-sha256", HeaderValue::from_str(&payload_hash).unwrap());
    headers.insert("x-amz-date", HeaderValue::from_str(&amz_date).unwrap());
    headers.insert(AUTHORIZATION, HeaderValue::from_str(&auth).unwrap());

    // 发起 GET 请求
    let client = Client::new();
    let resp = match client.get(&url).headers(headers).send().await {
        Ok(r) => r,
        Err(e) => {
            eprintln!("获取对象失败: {e}");
            exit(1);
        }
    };
    if !resp.status().is_success() {
        eprintln!("HTTP错误: {}", resp.status());
        exit(1);
    }

    // 读取并输出内容
    let bytes = match resp.bytes().await {
        Ok(b) => b,
        Err(e) => {
            eprintln!("读取对象内容失败: {e}");
            exit(1);
        }
    };
    let mut writer = tokio::io::stdout();
    let to_write = match transfer_size_bytes {
        Some(limit) => std::cmp::min(bytes.len(), limit),
        None => bytes.len(),
    };
    if writer.write_all(&bytes[..to_write]).await.is_err() {
        eprintln!("写入 stdout 失败");
        exit(1);
    }
}
