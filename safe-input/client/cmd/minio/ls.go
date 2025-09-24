package minio

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Print bucket headers
func printBucketHeaders() {
	fmt.Printf("| %-30s |\n", "Bucket Name")
	fmt.Println("|------------------------------|")
}

// Print object headers
func printObjectHeaders() {
	fmt.Printf("| %-30s | %-10s |\n", "Object Name", "Size")
	fmt.Println("|------------------------------|------------|")
}

// Handle ls command
func HandleLs(ctx context.Context, MinioClient *minio.Client, path string) {

	if len(path) == 0 {
		buckets, err := MinioClient.ListBuckets(ctx)
		if err != nil {
			fmt.Println("Error listing buckets:", err)
			return
		}

		printBucketHeaders()

		for _, bucket := range buckets {
			fmt.Printf("| %-30s |\n", bucket.Name)
		}
		return
	}

	parts := strings.SplitN(path, "/", 2)
	bucketName := parts[0]
	var prefix string
	if len(parts) > 1 {
		prefix = parts[1]
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectCh := MinioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false, // List all objects under the prefix
	})

	printObjectHeaders()

	for object := range objectCh {
		if object.Err != nil {
			fmt.Println("Error listing objects:", object.Err)
			return
		}
		fmt.Printf("| %-30s | %-10d |\n", object.Key, object.Size)
	}

}
