package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func HandleRbCommand(ctx context.Context, client *minio.Client, bucketName string) error {
	err := client.RemoveBucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("删除存储桶失败: %v", err)
	}
	fmt.Printf("存储桶 %s 已删除\n", bucketName)
	return nil
}
