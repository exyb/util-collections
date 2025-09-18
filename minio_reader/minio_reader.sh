#!/bin/bash
# MinIO S3 V4签名工具，支持 GET/PUT/DELETE/LIST 操作
# 用法:
#   get /bucket/path/to/file [transferSize] [--debug]
#   ls /bucket/path/ [--debug]
#   put /bucket/path/to/file localFile [--debug] [--progress]
#   rm /bucket/path/to/file [--debug]
export PS1='[${PWD##*/}]'
export PS4='+$LINENO: {${FUNCNAME[0]}} '

# 环境变量
ENDPOINT="${DATAFLUX_OSS_ADDRESS}"
ACCESS_KEY="${DATAFLUX_OSS_USERNAME}"
SECRET_KEY="${DATAFLUX_OSS_PASSWORD}"

if [[ -z "$ENDPOINT" || -z "$ACCESS_KEY" || -z "$SECRET_KEY" ]]; then
  echo "请设置 DATAFLUX_OSS_ADDRESS、DATAFLUX_OSS_USERNAME、DATAFLUX_OSS_PASSWORD 环境变量"
  exit 1
fi

REGION="us-east-1"

# 公共签名函数
function s3_sign_v4() {
  local method="$1"
  local uri="$2"
  local host="$3"
  local query="$4"
  local payload_hash="$5"
  local amz_date="$6"
  local date="$7"
  local signed_headers="$8"
  local canonical_headers="$9"

  local canonical_request="${method}
${uri}
${query}
${canonical_headers}
${signed_headers}
${payload_hash}"
  # 修正：header块后必须有一个空行
  canonical_request=$(echo "$canonical_request" | awk '
    BEGIN{h=0}
    NR==1{print;next}
    NR==2{print;next}
    NR==3{print;next}
    {
      if(h==0 && $0 ~ /^host:/){h=1}
      if(h==1 && $0 ~ /^host;x-amz-content-sha256;x-amz-date/){
        print ""
        h=2
      }
      print
    }
  ')
  local canonical_request_hash=$(printf "%s" "$canonical_request" | openssl dgst -sha256 -binary | xxd -p -c 256)
  local credential_scope="${date}/${REGION}/s3/aws4_request"
  local string_to_sign="AWS4-HMAC-SHA256
${amz_date}
${credential_scope}
${canonical_request_hash}"

  function hmac_sha256_bin() {
    local key_hex="$1"
    local data="$2"
    printf "%s" "$data" | openssl dgst -sha256 -mac HMAC -macopt hexkey:"$key_hex" -binary
  }
  function str2hex() {
    echo -n "$1" | xxd -p -c 256
  }

  local k_secret="AWS4${SECRET_KEY}"
  local k_date=$(hmac_sha256_bin "$(str2hex "$k_secret")" "$date" | xxd -p -c 256)
  local k_region=$(hmac_sha256_bin "$k_date" "$REGION" | xxd -p -c 256)
  local k_service=$(hmac_sha256_bin "$k_region" "s3" | xxd -p -c 256)
  local k_signing=$(hmac_sha256_bin "$k_service" "aws4_request" | xxd -p -c 256)
  local signature=$(printf "%s" "$string_to_sign" | openssl dgst -sha256 -mac HMAC -macopt hexkey:"$k_signing" | awk '{print $2}')

  local auth_header="AWS4-HMAC-SHA256 Credential=${ACCESS_KEY}/${credential_scope}, SignedHeaders=${signed_headers}, Signature=${signature}"
  echo "$auth_header"
}

# 公共头部生成
function build_headers() {
  local host="$1"
  local amz_date="$2"
  local payload_hash="$3"
  echo -e "host:${host}\nx-amz-content-sha256:${payload_hash}\nx-amz-date:${amz_date}\n"
}

# GET 下载
function s3_get() {
  # 参数解析
  local object_path=""
  local transfer_size=""
  local debug="0"
  for arg in "$@"; do
    if [[ "$arg" == "--debug" ]]; then
      debug="1"
    elif [[ -z "$object_path" ]]; then
      object_path="$arg"
    elif [[ -z "$transfer_size" ]]; then
      transfer_size="$arg"
    fi
  done
  # 规范化 object_path，去除前导斜杠
  object_path="${object_path#/}"
  local bucket="${object_path%%/*}"
  local key="${object_path#*/}"
  local uri="/${bucket}/${key}"
  local url="${ENDPOINT%/}/$bucket/$key"
  local proto=$(echo "$ENDPOINT" | grep -oE '^https?')
  local hostport=$(echo "$ENDPOINT" | sed -E 's#^https?://##')
  local host=$(echo "$hostport" | cut -d: -f1)
  local port=$(echo "$hostport" | cut -s -d: -f2)
  if [[ "$proto" == "http" && "$port" == "80" ]]; then
    host="$host"
  elif [[ "$proto" == "https" && "$port" == "443" ]]; then
    host="$host"
  elif [[ -n "$port" ]]; then
    host="$host:$port"
  fi
  local amz_date=$(date -u +"%Y%m%dT%H%M%SZ")
  local date=$(date -u +"%Y%m%d")
  local query=""
  local payload_hash="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  local canonical_headers=$(build_headers "$host" "$amz_date" "$payload_hash")
  local signed_headers="host;x-amz-content-sha256;x-amz-date"
  local auth_header=$(s3_sign_v4 "GET" "$uri" "$host" "$query" "$payload_hash" "$amz_date" "$date" "$signed_headers" "$canonical_headers")

  if [[ "$debug" == "1" ]]; then
    echo "调试信息:"
    echo "URL: $url"
    echo "Host: $host"
    echo "amz_date: $amz_date"
    echo "date: $date"
    echo "Authorization: $auth_header"
    echo "x-amz-content-sha256: $payload_hash"
    echo "canonical_request:"
    echo "------------------"
    echo -e "GET\n${uri}\n${query}\n${canonical_headers}\n${signed_headers}\n${payload_hash}"
    echo "------------------"
  fi

  CURL_OPTS=(
    # -s
    -H "Host: $host"
    -H "x-amz-content-sha256: $payload_hash"
    -H "x-amz-date: $amz_date"
    -H "Authorization: $auth_header"
    -H "Accept-Encoding: identity"
    -H "Connection: close"
    -H "User-Agent: aws-cli/1.16.0"
  )
  if [[ -n "$transfer_size" ]]; then
    CURL_OPTS+=(-H "Range: bytes=0-$(($transfer_size * 1024 - 1))")
  fi
  CURL_OPTS+=("$url")
  curl "${CURL_OPTS[@]}"
}

# 通用 S3 列表请求
function s3_list_request() {
  # 参数: $1=bucket, $2=prefix, $3=query, $4=debug, $5=uri
  local bucket="$1"
  local prefix="$2"
  local query="$3"
  local debug="$4"
  local uri="$5"
  local proto=$(echo "$ENDPOINT" | grep -oE '^https?')
  local hostport=$(echo "$ENDPOINT" | sed -E 's#^https?://##')
  local host=$(echo "$hostport" | cut -d: -f1)
  local port=$(echo "$hostport" | cut -s -d: -f2)
  if [[ "$proto" == "http" && "$port" == "80" ]]; then
    host="$host"
  elif [[ "$proto" == "https" && "$port" == "443" ]]; then
    host="$host"
  elif [[ -n "$port" ]]; then
    host="$host:$port"
  fi
  local amz_date=$(date -u +"%Y%m%dT%H%M%SZ")
  local date=$(date -u +"%Y%m%d")
  local url="${ENDPOINT%/}/$bucket?${query}"
  local payload_hash="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  local canonical_headers=$(build_headers "$host" "$amz_date" "$payload_hash")
  local signed_headers="host;x-amz-content-sha256;x-amz-date"
  local auth_header=$(s3_sign_v4 "GET" "$uri" "$host" "$query" "$payload_hash" "$amz_date" "$date" "$signed_headers" "$canonical_headers")

  if [[ "$debug" == "1" ]]; then
    echo "调试信息:"
    echo "URL: $url"
    echo "Host: $host"
    echo "amz_date: $amz_date"
    echo "date: $date"
    echo "Authorization: $auth_header"
    echo "x-amz-content-sha256: $payload_hash"
    echo "canonical_request:"
    echo "------------------"
    echo -e "GET\n${uri}\n${query}\n${canonical_headers}\n${signed_headers}\n${payload_hash}"
    echo "------------------"
  fi

  curl -s -H "Host: $host" -H "x-amz-content-sha256: $payload_hash" -H "x-amz-date: $amz_date" -H "Authorization: $auth_header" -H "Accept-Encoding: identity" -H "Connection: close" -H "User-Agent: aws-cli/1.16.0" "$url"
}

# LIST 列表
function s3_ls() {
  # 参数解析
  local object_path=""
  local debug="0"
  local level=""
  local prefix_match="0"
  for ((i=1;i<=$#;i++)); do
    arg="${!i}"
    if [[ "$arg" == "--debug" ]]; then
      debug="1"
    elif [[ "$arg" == "-l" ]]; then
      nexti=$((i+1))
      level="${!nexti}"
      ((i++))
    elif [[ "$arg" == "--prefix-match" ]]; then
      prefix_match="1"
    elif [[ -z "$object_path" ]]; then
      object_path="$arg"
    fi
  done
  object_path="${object_path#/}"
  local bucket="${object_path%%/*}"
  local prefix="${object_path#*/}"
  local uri="/${bucket}"
  local proto=$(echo "$ENDPOINT" | grep -oE '^https?')
  local hostport=$(echo "$ENDPOINT" | sed -E 's#^https?://##')
  local host=$(echo "$hostport" | cut -d: -f1)
  local port=$(echo "$hostport" | cut -s -d: -f2)
  if [[ "$proto" == "http" && "$port" == "80" ]]; then
    host="$host"
  elif [[ "$proto" == "https" && "$port" == "443" ]]; then
    host="$host"
  elif [[ -n "$port" ]]; then
    host="$host:$port"
  fi
  local amz_date=$(date -u +"%Y%m%dT%H%M%SZ")
  local date=$(date -u +"%Y%m%d")
  local query="list-type=2&prefix=${prefix}"
  local url="${ENDPOINT%/}/$bucket?${query}"
  local payload_hash="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  local canonical_headers=$(build_headers "$host" "$amz_date" "$payload_hash")
  local signed_headers="host;x-amz-content-sha256;x-amz-date"
  local auth_header=$(s3_sign_v4 "GET" "$uri" "$host" "$query" "$payload_hash" "$amz_date" "$date" "$signed_headers" "$canonical_headers")

  # 前缀匹配增强
  if [[ "$prefix_match" == "1" ]]; then
    # 自动 list 上一级目录，对结果进行前缀筛选
    local parent_prefix="${prefix%/*}"
    local match_name="${prefix##*/}"
    if [[ -n "$parent_prefix" ]]; then
      local parent_query="list-type=2&prefix=${parent_prefix}"
      local parent_uri="/${bucket}"
      if [[ "$debug" == "1" ]]; then
        echo "自动前缀匹配: 请求上级目录"
        echo "Parent Query: $parent_query"
        echo "筛选前缀: $match_name"
      fi
      local curl_raw
      curl_raw=$(s3_list_request "$bucket" "$parent_prefix" "$parent_query" "$debug" "$parent_uri")
      if [[ "$debug" == "1" ]]; then
        echo "curl 原始输出（前1000字节预览）:"
        echo "${curl_raw:0:1000}"
      fi
      local raw_keys
      raw_keys=$(printf '%s\n' "$curl_raw" | grep -oP '(?<=<Key>).*?(?=</Key>)')
      # if [[ "$debug" == "1" ]]; then
      #   echo "筛选前内容:"
      #   printf '%s\n' "$raw_keys"
      # fi
      printf '%s\n' "$raw_keys" | awk -v m="$match_name" -F'/' '{ if($(NF) ~ "^"m) print $0 }'
    else
      # 如果没有上一级，直接 list 当前前缀
      local query="list-type=2&prefix=${prefix}"
      local uri="/${bucket}"
      if [[ "$debug" == "1" ]]; then
        echo "自动前缀匹配: 无上级目录，直接请求"
        echo "Query: $query"
      fi
      s3_list_request "$bucket" "$prefix" "$query" "$debug" "$uri" | grep -oP '(?<=<Key>).*?(?=</Key>)'
    fi
    return
  fi


  if [[ "$debug" == "1" ]]; then
    echo "调试信息:"
  fi
  s3_list_request "$bucket" "$prefix" "$query" "$debug" "$uri" | grep -oP '(?<=<Key>).*?(?=</Key>)'
  # 目录限定选项
  local only_dir="0"
  for arg in "$@"; do
    if [[ "$arg" == "--dironly" ]]; then
      only_dir="1"
    fi
  done

  # 前缀匹配增强
  if [[ "$prefix_match" == "1" ]]; then
    # 自动 list 上一级目录，对结果进行前缀筛选
    local parent_prefix="${prefix%/*}"
    local match_name="${prefix##*/}"
    if [[ -n "$parent_prefix" ]]; then
      local parent_query="list-type=2&prefix=${parent_prefix}"
      local parent_url="${ENDPOINT%/}/$bucket?${parent_query}"
      if [[ "$debug" == "1" ]]; then
        echo "自动前缀匹配: 请求上级目录"
        echo "Parent URL: $parent_url"
        echo "筛选前缀: $match_name"
      fi
      curl -s -H "Host: $host" -H "x-amz-content-sha256: $payload_hash" -H "x-amz-date: $amz_date" -H "Authorization: $auth_header" -H "Accept-Encoding: identity" -H "Connection: close" -H "User-Agent: aws-cli/1.16.0" "$parent_url" | grep -oP '(?<=<Key>).*?(?=</Key>)' | awk -v m="$match_name" -F'/' '{ if($(NF) ~ "^"m) print $0 }'
    else
      # 如果没有上一级，直接 list 当前前缀
      local query="list-type=2&prefix=${prefix}"
      local url="${ENDPOINT%/}/$bucket?${query}"
      if [[ "$debug" == "1" ]]; then
        echo "自动前缀匹配: 无上级目录，直接请求"
        echo "URL: $url"
      fi
      curl -s -H "Host: $host" -H "x-amz-content-sha256: $payload_hash" -H "x-amz-date: $amz_date" -H "Authorization: $auth_header" -H "Accept-Encoding: identity" -H "Connection: close" -H "User-Agent: aws-cli/1.16.0" "$url" | grep -oP '(?<=<Key>).*?(?=</Key>)'
    fi
    return
  fi

  # 递归级别和目录限定处理
  if [[ -n "$level" || "$only_dir" == "1" ]]; then
    CURL_OPTS=(
      -s
      -H "Host: $host"
      -H "x-amz-content-sha256: $payload_hash"
      -H "x-amz-date: $amz_date"
      -H "Authorization: $auth_header"
      -H "Accept-Encoding: identity"
      -H "Connection: close"
      -H "User-Agent: aws-cli/1.16.0"
      "$url"
    )
    curl "${CURL_OPTS[@]}" | grep -oP '(?<=<Key>).*?(?=</Key>)' | awk -F'/' -v l="$level" -v prefix="$prefix" -v dironly="$only_dir" '
      {
        n=split($0, arr, "/");
        # 只输出当前目录下的对象（不递归子目录）
        if(dironly=="1") {
          if(n==split(prefix, _, "/")+1) print $0
        } else if(l==0 || n<=l) {
          print $0
        }
      }
    '
  else
    CURL_OPTS=(
      -s
      -H "Host: $host"
      -H "x-amz-content-sha256: $payload_hash"
      -H "x-amz-date: $amz_date"
      -H "Authorization: $auth_header"
      -H "Accept-Encoding: identity"
      -H "Connection: close"
      -H "User-Agent: aws-cli/1.16.0"
      "$url"
    )
    curl "${CURL_OPTS[@]}" | grep -oP '(?<=<Key>).*?(?=</Key>)'
  fi
}

# PUT 上传
function s3_put() {
  # 参数解析
  local object_path=""
  local local_file=""
  local debug="0"
  local progress="0"
  for arg in "$@"; do
    if [[ "$arg" == "--debug" ]]; then
      debug="1"
    elif [[ "$arg" == "--progress" ]]; then
      progress="1"
    elif [[ -z "$object_path" ]]; then
      object_path="$arg"
    elif [[ -z "$local_file" ]]; then
      local_file="$arg"
    fi
  done
  object_path="${object_path#/}"
  local bucket="${object_path%%/*}"
  local key="${object_path#*/}"
  local uri="/${bucket}/${key}"
  local url="${ENDPOINT%/}/$bucket/$key"
  local proto=$(echo "$ENDPOINT" | grep -oE '^https?')
  local hostport=$(echo "$ENDPOINT" | sed -E 's#^https?://##')
  local host=$(echo "$hostport" | cut -d: -f1)
  local port=$(echo "$hostport" | cut -s -d: -f2)
  if [[ "$proto" == "http" && "$port" == "80" ]]; then
    host="$host"
  elif [[ "$proto" == "https" && "$port" == "443" ]]; then
    host="$host"
  elif [[ -n "$port" ]]; then
    host="$host:$port"
  fi
  local amz_date=$(date -u +"%Y%m%dT%H%M%SZ")
  local date=$(date -u +"%Y%m%d")
  local query=""
  local payload_hash=$(openssl dgst -sha256 -binary "$local_file" | xxd -p -c 256)
  local canonical_headers=$(build_headers "$host" "$amz_date" "$payload_hash")
  local signed_headers="host;x-amz-content-sha256;x-amz-date"
  local auth_header=$(s3_sign_v4 "PUT" "$uri" "$host" "$query" "$payload_hash" "$amz_date" "$date" "$signed_headers" "$canonical_headers")

  if [[ "$debug" == "1" ]]; then
    echo "调试信息:"
    echo "URL: $url"
    echo "Host: $host"
    echo "amz_date: $amz_date"
    echo "date: $date"
    echo "Authorization: $auth_header"
    echo "x-amz-content-sha256: $payload_hash"
    echo "canonical_request:"
    echo "------------------"
    echo -e "PUT\n${uri}\n${query}\n${canonical_headers}\n${signed_headers}\n${payload_hash}"
    echo "------------------"
  fi

  if [[ "$progress" == "1" ]]; then
    CURL_OPTS=(
      -H "Host: $host"
      -H "x-amz-content-sha256: $payload_hash"
      -H "x-amz-date: $amz_date"
      -H "Authorization: $auth_header"
      -H "Accept-Encoding: identity"
      -H "Connection: close"
      -H "User-Agent: aws-cli/1.16.0"
      -T "$local_file"
      "$url"
    )
    curl "${CURL_OPTS[@]}" 2>/dev/tty
  else
    CURL_OPTS=(
      -s
      -H "Host: $host"
      -H "x-amz-content-sha256: $payload_hash"
      -H "x-amz-date: $amz_date"
      -H "Authorization: $auth_header"
      -H "Accept-Encoding: identity"
      -H "Connection: close"
      -H "User-Agent: aws-cli/1.16.0"
      -T "$local_file"
      "$url"
    )
    curl "${CURL_OPTS[@]}"
  fi
}

# DELETE 删除
function s3_rm() {
  # 参数解析
  local object_path=""
  local debug="0"
  for arg in "$@"; do
    if [[ "$arg" == "--debug" ]]; then
      debug="1"
    elif [[ -z "$object_path" ]]; then
      object_path="$arg"
    fi
  done
  object_path="${object_path#/}"
  local bucket="${object_path%%/*}"
  local key="${object_path#*/}"
  local uri="/${bucket}/${key}"
  local url="${ENDPOINT%/}/$bucket/$key"
  local proto=$(echo "$ENDPOINT" | grep -oE '^https?')
  local hostport=$(echo "$ENDPOINT" | sed -E 's#^https?://##')
  local host=$(echo "$hostport" | cut -d: -f1)
  local port=$(echo "$hostport" | cut -s -d: -f2)
  if [[ "$proto" == "http" && "$port" == "80" ]]; then
    host="$host"
  elif [[ "$proto" == "https" && "$port" == "443" ]]; then
    host="$host"
  elif [[ -n "$port" ]]; then
    host="$host:$port"
  fi
  local amz_date=$(date -u +"%Y%m%dT%H%M%SZ")
  local date=$(date -u +"%Y%m%d")
  local query=""
  local payload_hash="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  local canonical_headers=$(build_headers "$host" "$amz_date" "$payload_hash")
  local signed_headers="host;x-amz-content-sha256;x-amz-date"
  local auth_header=$(s3_sign_v4 "DELETE" "$uri" "$host" "$query" "$payload_hash" "$amz_date" "$date" "$signed_headers" "$canonical_headers")

  if [[ "$debug" == "1" ]]; then
    echo "调试信息:"
    echo "URL: $url"
    echo "Host: $host"
    echo "amz_date: $amz_date"
    echo "date: $date"
    echo "Authorization: $auth_header"
    echo "x-amz-content-sha256: $payload_hash"
    echo "canonical_request:"
    echo "------------------"
    echo -e "DELETE\n${uri}\n${query}\n${canonical_headers}\n${signed_headers}\n${payload_hash}"
    echo "------------------"
  fi

  CURL_OPTS=(
    -s
    -X DELETE
    -H "Host: $host"
    -H "x-amz-content-sha256: $payload_hash"
    -H "x-amz-date: $amz_date"
    -H "Authorization: $auth_header"
    -H "Accept-Encoding: identity"
    -H "Connection: close"
    -H "User-Agent: aws-cli/1.16.0"
    "$url"
  )
  curl "${CURL_OPTS[@]}"
}

# 主入口
cmd="$1"
shift
case "$cmd" in
  get)
    s3_get "$@"
    ;;
  ls)
    s3_ls "$@"
    ;;
  put)
    s3_put "$@"
    ;;
  rm)
    s3_rm "$@"
    ;;
  *)
    echo "用法:"
    echo "  get /bucket/path/to/file [transferSize] [--debug]"
    echo "  ls /bucket/path/ [--debug]"
    echo "  put /bucket/path/to/file localFile [--debug] [--progress]"
    echo "  rm /bucket/path/to/file [--debug]"
    exit 1
    ;;
esac
