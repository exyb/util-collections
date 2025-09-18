use std::env;
use std::process::exit;
use std::io::{self, Write};
use clap::Parser;
use aws_sdk_s3::Client;
use aws_config::meta::region::RegionProviderChain;
use aws_sdk_s3::primitives::ByteStream;
use aws_sdk_s3::config::Region;
use dotenvy::dotenv;

/// 命令行参数结构体
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// MinIO对象路径，格式为 /bucket/object/path
    object_path: String,
    /// 读取大小，单位KB，留空则读取完整文件
    transfer_size: Option<usize>,
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

    // 构建 S3 客户端
    let region_provider = RegionProviderChain::first_try(Region::new("us-east-1"));
    let config = aws_config::defaults(aws_config::BehaviorVersion::latest())
        .region(region_provider)
        .endpoint_url(endpoint)
        .credentials_provider(aws_sdk_s3::config::Credentials::new(
            access_key,
            secret_key,
            None,
            None,
            "minio_reader",
        ))
        .load()
        .await;
    let client = Client::new(&config);

    // 获取对象
    let resp = match client.get_object()
        .bucket(bucket)
        .key(key)
        .send()
        .await {
        Ok(r) => r,
        Err(e) => {
            eprintln!("获取对象失败: {e}");
            exit(1);
        }
    };

    let mut stream = resp.body.into_async_read();
    let mut writer = io::stdout();
    let mut buf = [0u8; 8192];
    let mut total = 0usize;

    loop {
        let to_read = match transfer_size_bytes {
            Some(limit) => {
                if total >= limit {
                    break;
                }
                std::cmp::min(buf.len(), limit - total)
            }
            None => buf.len(),
        };
        let n = match tokio::io::AsyncReadExt::read(&mut stream, &mut buf[..to_read]).await {
            Ok(0) => break,
            Ok(n) => n,
            Err(e) => {
                eprintln!("读取对象内容失败: {e}");
                exit(1);
            }
        };
        if writer.write_all(&buf[..n]).is_err() {
            eprintln!("写入 stdout 失败");
            exit(1);
        }
        total += n;
    }
}
