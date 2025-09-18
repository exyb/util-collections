//! 极小依赖 MinIO(S3兼容)对象部分下载工具
//! 仅依赖 minreq、hmac、sha2，全部同步实现

use std::env;
use std::process::exit;
use std::io::{self, Write};
use std::time::{SystemTime, UNIX_EPOCH};
use hmac::{Hmac, Mac};
use sha2::{Sha256, Digest};
use minreq;

/// 计算SHA256十六进制字符串
fn sha256_hex(data: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(data);
    format!("{:x}", hasher.finalize())
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
    amz_date: &str,
    date: &str,
) -> String {
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
    format!(
        "AWS4-HMAC-SHA256 Credential={}/{}, SignedHeaders={}, Signature={}",
        access_key, credential_scope, signed_headers, signature
    )
}

fn main() {
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
    let args: Vec<String> = env::args().collect();
    if args.len() < 2 {
        eprintln!("用法: minio_reader /bucket/object/path [transfer_size_KB] [--debug]");
        exit(1);
    }
    // 判断是否有 --debug
    let mut debug = false;
    let mut object_path = "";
    let mut transfer_size_bytes = None;
    for arg in &args[1..] {
        if arg == "--debug" {
            debug = true;
        }
    }
    // 解析 object_path 和 transfer_size
    let mut arg_iter = args[1..].iter().filter(|a| *a != "--debug");
    object_path = match arg_iter.next() {
        Some(s) => s.trim_start_matches('/'),
        None => {
            eprintln!("objectPath 缺失，需为 /bucket/object/path");
            exit(1);
        }
    };
    let mut parts = object_path.splitn(2, '/');
    let bucket = parts.next().unwrap_or("");
    let key = parts.next().unwrap_or("");
    if bucket.is_empty() || key.is_empty() {
        eprintln!("objectPath 格式错误，需为 /bucket/object/path");
        exit(1);
    }
    transfer_size_bytes = match arg_iter.next() {
        Some(s) => s.parse::<usize>().ok().map(|kb| kb * 1024),
        None => None,
    };

    // 构造 S3 GET Object URL
    let region = "us-east-1"; // MinIO 默认兼容
    let url = format!("{}/{}/{}", endpoint.trim_end_matches('/'), bucket, key);

    // S3 V4 签名
    let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_secs() as i64;
    let tm = unsafe {
        let mut out = std::mem::zeroed();
        libc::gmtime_r(&now, &mut out);
        out
    };
    let amz_date = format!(
        "{:04}{:02}{:02}T{:02}{:02}{:02}Z",
        tm.tm_year + 1900,
        tm.tm_mon + 1,
        tm.tm_mday,
        tm.tm_hour,
        tm.tm_min,
        tm.tm_sec
    );
    let date = format!(
        "{:04}{:02}{:02}",
        tm.tm_year + 1900,
        tm.tm_mon + 1,
        tm.tm_mday
    );
    let uri = format!("/{}/{}", bucket, key);
    let host = endpoint.trim_start_matches("http://").trim_start_matches("https://");
    let query = "";
    let payload_hash = sha256_hex(b"");
    let auth = s3_sign_v4(
        "GET",
        &uri,
        host,
        region,
        &access_key,
        &secret_key,
        query,
        &payload_hash,
        &amz_date,
        &date,
    );

    // 输出调试信息（仅在 --debug 时）
    if debug {
        eprintln!("调试信息:");
        eprintln!("URL: {}", url);
        eprintln!("Host: {}", host);
        eprintln!("amz_date: {}", amz_date);
        eprintln!("date: {}", date);
        eprintln!("Authorization: {}", auth);
        eprintln!("x-amz-content-sha256: {}", payload_hash);
    }

    // 构造请求
    let resp = minreq::get(&url)
        // .with_header("Host", host) // 先去掉 Host 头，使用 minreq 默认
        .with_header("x-amz-content-sha256", &payload_hash)
        .with_header("x-amz-date", &amz_date)
        .with_header("Authorization", &auth)
        .with_header("Accept-Encoding", "identity")
        .with_header("Connection", "close")
        .send();

    let resp = match resp {
        Ok(r) => r,
        Err(e) => {
            eprintln!("获取对象失败: {e}");
            exit(1);
        }
    };
    if resp.status_code != 200 {
        eprintln!("HTTP错误: {}", resp.status_code);
        if debug {
            eprintln!("响应内容: {}", String::from_utf8_lossy(resp.as_bytes()));
        }
        exit(1);
    }

    // 读取并输出内容
    let bytes = resp.as_bytes();
    let to_write = match transfer_size_bytes {
        Some(limit) => std::cmp::min(bytes.len(), limit),
        None => bytes.len(),
    };
    let stdout = io::stdout();
    let mut handle = stdout.lock();
    if handle.write_all(&bytes[..to_write]).is_err() {
        eprintln!("写入 stdout 失败");
        exit(1);
    }
}
