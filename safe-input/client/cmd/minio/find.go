package minio

import (
	"context"
	"fmt"
	"regexp"

	"github.com/minio/minio-go/v7"
)

func HandleFindCommand(ctx context.Context, client *minio.Client, bucketName, pattern string) error {

	objects := client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	// 使用正则表达式匹配对象名
	matched, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Println("Invalid pattern:", err)
		return nil
	}

	for object := range objects {
		if object.Err != nil {
			fmt.Println("Error listing object:", object.Err)
			return nil
		}

		if matched.MatchString(object.Key) {
			fmt.Println("Matched object:", object.Key)
		}
	}

	return nil
}
