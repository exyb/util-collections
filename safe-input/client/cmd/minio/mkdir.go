package minio

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Function to handle 'mkdir' command
func HandleMkdirCommand(ctx context.Context, MinioClient *minio.Client, path string, recursive bool) {

	// Extract bucket name and directory path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		fmt.Println("Invalid path. Usage: mkdir <bucket>/<directory>")
		return
	}
	bucketName, dirPath := parts[0], parts[1]

	// If recursive, ensure all parent directories exist
	if recursive {
		if err := ensureParentDirs(ctx, MinioClient, bucketName, dirPath); err != nil {
			fmt.Printf("Failed to create directories: %v\n", err)
			return
		}
	} else {
		// Create a single directory (simulate by creating a dummy object)
		_, err := MinioClient.PutObject(ctx, bucketName, dirPath+"/", nil, 0, minio.PutObjectOptions{})
		if err != nil {
			fmt.Printf("Failed to create directory: %v\n", err)
		} else {
			fmt.Println("Directory created:", dirPath)
		}
	}
}

// Ensure all parent directories exist
func ensureParentDirs(ctx context.Context, MinioClient *minio.Client, bucketName, dirPath string) error {

	parts := strings.Split(dirPath, "/")
	for i := 1; i <= len(parts); i++ {
		prefix := strings.Join(parts[:i], "/") + "/"
		if _, err := MinioClient.PutObject(ctx, bucketName, prefix, nil, 0, minio.PutObjectOptions{}); err != nil {
			return err
		}
	}
	fmt.Println("Directories created:", dirPath)
	return nil
}
