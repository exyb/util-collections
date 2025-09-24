package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func HandleMvCommand(ctx context.Context, client *minio.Client, srcBucket, srcObject, destBucket, destObject string) error {
	_, err := client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: destBucket, Object: destObject,
	}, minio.CopySrcOptions{
		Bucket: srcBucket, Object: srcObject,
	})
	if err != nil {
		return fmt.Errorf("移动对象失败: %v", err)
	}

	err = client.RemoveObject(ctx, srcBucket, srcObject, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("删除源对象失败: %v", err)
	}

	fmt.Printf("对象已从 %s/%s 移动到 %s/%s\n", srcBucket, srcObject, destBucket, destObject)
	return nil
}
