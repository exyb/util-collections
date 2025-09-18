#!/bin/bash
# 用最基础工具 curl + openssl 实现 S3/MinIO V4签名的部分对象下载
# 用法: ./minio_reader.sh /bucket/object/path [transfer_size_KB] [--debug]

# 环境变量
ENDPOINT="${DATAFLUX_OSS_ADDRESS}"
ACCESS_KEY="${DATAFLUX_OSS_USERNAME}"
SECRET_KEY="${DATAFLUX_OSS_PASSWORD}"

if [[ -z "$ENDPOINT" || -z "$ACCESS_KEY" || -z "$SECRET_KEY" ]]; then
  echo "请设置 DATAFLUX_OSS_ADDRESS、DATAFLUX_OSS_USERNAME、DATAFLUX_OSS_PASSWORD 环境变量"
  exit 1
fi

# 参数解析
DEBUG=0
for arg in "$@"; do
  if [[ "$arg" == "--debug" ]]; then
    DEBUG=1
  fi
done

ARGS=()
for arg in "$@"; do
  if [[ "$arg" != "--debug" ]]; then
    ARGS+=("$arg")
  fi
done

if [[ ${#ARGS[@]} -lt 1 ]]; then
  echo "用法: $0 /bucket/object/path [transfer_size_KB] [--debug]"
  exit 1
fi

OBJECT_PATH="${ARGS[0]#/}"
BUCKET="${OBJECT_PATH%%/*}"
KEY="${OBJECT_PATH#*/}"
if [[ -z "$BUCKET" || -z "$KEY" ]]; then
  echo "objectPath 格式错误，需为 /bucket/object/path"
  exit 1
fi

TRANSFER_SIZE=""
if [[ ${#ARGS[@]} -ge 2 ]]; then
  TRANSFER_SIZE="${ARGS[1]}"
fi

REGION="us-east-1"
URL="${ENDPOINT%/}/$BUCKET/$KEY"

# 时间戳
AMZ_DATE=$(date -u +"%Y%m%dT%H%M%SZ")
DATE=$(date -u +"%Y%m%d")

# S3 V4签名相关
URI="/$BUCKET/$KEY"
# 处理 host: 端口规范（80/443时不带端口）
proto=$(echo "$ENDPOINT" | grep -oE '^https?')
hostport=$(echo "$ENDPOINT" | sed -E 's#^https?://##')
host=$(echo "$hostport" | cut -d: -f1)
port=$(echo "$hostport" | cut -s -d: -f2)
if [[ "$proto" == "http" && "$port" == "80" ]]; then
  HOST="$host"
elif [[ "$proto" == "https" && "$port" == "443" ]]; then
  HOST="$host"
elif [[ -n "$port" ]]; then
  HOST="$host:$port"
else
  HOST="$host"
fi

QUERY=""
PAYLOAD_HASH="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

CANONICAL_HEADERS="host:${HOST}
x-amz-content-sha256:${PAYLOAD_HASH}
x-amz-date:${AMZ_DATE}
"
SIGNED_HEADERS="host;x-amz-content-sha256;x-amz-date"

# Range 只加到 curl header，不参与签名

CANONICAL_REQUEST="GET
${URI}
${QUERY}
${CANONICAL_HEADERS}
${SIGNED_HEADERS}
${PAYLOAD_HASH}"
# 保证 canonical request 的 header 块每行一个header，header块后有一个空行

CANONICAL_REQUEST_HASH=$(printf "%s" "$CANONICAL_REQUEST" | openssl dgst -sha256 -binary | xxd -p -c 256)
CREDENTIAL_SCOPE="${DATE}/${REGION}/s3/aws4_request"
STRING_TO_SIGN="AWS4-HMAC-SHA256
${AMZ_DATE}
${CREDENTIAL_SCOPE}
${CANONICAL_REQUEST_HASH}"

# 计算签名，全部用二进制key
function hmac_sha256_bin() {
  local key_hex="$1"
  local data="$2"
  printf "%s" "$data" | openssl dgst -sha256 -mac HMAC -macopt hexkey:"$key_hex" -binary
}
function str2hex() {
  echo -n "$1" | xxd -p -c 256
}

K_SECRET="AWS4${SECRET_KEY}"
K_DATE=$(hmac_sha256_bin "$(str2hex "$K_SECRET")" "$DATE" | xxd -p -c 256)
K_REGION=$(hmac_sha256_bin "$K_DATE" "$REGION" | xxd -p -c 256)
K_SERVICE=$(hmac_sha256_bin "$K_REGION" "s3" | xxd -p -c 256)
K_SIGNING=$(hmac_sha256_bin "$K_SERVICE" "aws4_request" | xxd -p -c 256)
SIGNATURE=$(printf "%s" "$STRING_TO_SIGN" | openssl dgst -sha256 -mac HMAC -macopt hexkey:"$K_SIGNING" | awk '{print $2}')

AUTH_HEADER="AWS4-HMAC-SHA256 Credential=${ACCESS_KEY}/${CREDENTIAL_SCOPE}, SignedHeaders=${SIGNED_HEADERS}, Signature=${SIGNATURE}"

if [[ $DEBUG -eq 1 ]]; then
  echo "调试信息:"
  echo "URL: $URL"
  echo "Host: $HOST"
  echo "amz_date: $AMZ_DATE"
  echo "date: $DATE"
  echo "Authorization: $AUTH_HEADER"
  echo "x-amz-content-sha256: $PAYLOAD_HASH"
  echo "canonical_request:"
  echo "------------------"
  echo "$CANONICAL_REQUEST"
  echo "------------------"
  echo "string_to_sign:"
  echo "------------------"
  echo "$STRING_TO_SIGN"
  echo "------------------"
fi

# 构造 curl 命令
CURL_OPTS=(
  -s
  -H "Host: $HOST"
  -H "x-amz-content-sha256: $PAYLOAD_HASH"
  -H "x-amz-date: $AMZ_DATE"
  -H "Authorization: $AUTH_HEADER"
  -H "Accept-Encoding: identity"
  -H "Connection: close"
  -H "User-Agent: aws-cli/1.16.0"
)

if [[ -n "$TRANSFER_SIZE" ]]; then
  CURL_OPTS+=(-H "Range: bytes=0-$(($TRANSFER_SIZE * 1024 - 1))")
fi

CURL_OPTS+=("$URL")

curl "${CURL_OPTS[@]}"
