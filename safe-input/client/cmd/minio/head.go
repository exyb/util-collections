package minio

import (
	"bufio"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func HandleHeadCommand(ctx context.Context, client *minio.Client, bucketName, objectName string, lineCount int) {

	// 使用 MinIO SDK 获取对象
	object, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		fmt.Println("Error getting object:", err)
		return
	}
	defer object.Close()

	// 逐行读取对象内容
	scanner := bufio.NewScanner(object)
	lineNum := 0

	for scanner.Scan() {
		fmt.Println(scanner.Text()) // 打印每一行
		lineNum++
		if lineNum >= lineCount { // 当达到指定行数时停止
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading object:", err)
	}
}
