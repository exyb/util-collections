# 启动中转服务器

``` shell
./transfer-station [-p port]
```

# 使用方法

up aa file1 file2
dl aa

``` shell
up() {
	[ $# -le 2 ] && echo "Usage: path files..." && return
	local path=$1
	shift
	tar -cf - "$@" |curl -X POST --data-binary @- ip:port/${path}
}
dl() {
	local path=$1
	test -z "$path" && return
	curl -o- ip:port/${path}|tar -xf -
}
```
