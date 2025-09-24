package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func HandleMbCommand(ctx context.Context, client *minio.Client, bucketName string) error {
	err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("创建存储桶失败: %v", err)
	}
	fmt.Printf("存储桶 %s 已成功创建\n", bucketName)
	return nil
}
