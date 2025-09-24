package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func HandleTreeCommand(ctx context.Context, client *minio.Client, bucketName, objectPrefix string) error {
	objectCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    objectPrefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return fmt.Errorf("遍历对象失败: %v", object.Err)
		}
		fmt.Printf("|-- %s\n", object.Key)
	}
	return nil
}
