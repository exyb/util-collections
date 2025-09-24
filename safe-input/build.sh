export CGO_ENABLED=0
export GOOS=linux
export GOARCH=arm64

go build -ldflags "-s -w" -o safe-input.arm64
upx safe-input.arm64

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

go build -ldflags "-s -w" -o safe-input.amd64
upx safe-input.amd64
