package minio

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

func HandleCatCommand(ctx context.Context, client *minio.Client, bucketName, objectName string) {

	// 使用 MinIO SDK 获取对象
	object, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		fmt.Println("Error getting object:", err)
		return
	}
	defer object.Close()

	// 读取对象的全部内容并输出
	data, err := io.ReadAll(object)
	if err != nil {
		fmt.Println("Error reading object:", err)
		return
	}

	fmt.Println(string(data)) // 输出对象内容
}
