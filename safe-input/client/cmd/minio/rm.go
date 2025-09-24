package minio

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
)

func HandleRmCommand(ctx context.Context, client *minio.Client, bucketName, objectPrefix string, recursive bool, force bool) error {
	if recursive {
		objectCh := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
			Prefix:    objectPrefix,
			Recursive: true,
		})

		for object := range objectCh {
			if object.Err != nil {
				return fmt.Errorf("遍历对象失败: %v", object.Err)
			}
			err := client.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{
				ForceDelete: force,
			})
			if err != nil {
				return fmt.Errorf("删除对象 %s 失败: %v", object.Key, err)
			}
			fmt.Printf("删除对象 %s 成功\n", object.Key)
		}
	} else {
		err := client.RemoveObject(ctx, bucketName, objectPrefix, minio.RemoveObjectOptions{
			ForceDelete: force,
		})
		if err != nil {
			return fmt.Errorf("删除对象 %s 失败: %v", objectPrefix, err)
		}
		fmt.Printf("对象 %s 已删除\n", objectPrefix)
	}
	return nil
}
