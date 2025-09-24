package minio

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
)

// Handle cp command
func HandleCpCommand(ctx context.Context, MinioClient *minio.Client, args []string) {
	log.Printf("args: %s", args)

	if len(args) < 2 {
		fmt.Println("Usage: cp [-r] <source> <destination>")
		return
	}

	recursive := false
	if args[0] == "-r" {
		recursive = true
		args = args[1:]
	}

	if len(args) < 2 {
		fmt.Println("Usage: cp [-r] <source> <destination>")
		return
	}

	source := args[0]
	destination := args[1]
	log.Printf("source: %s, destination: %s", source, destination)

	// Determine if source or destination is local or remote
	if strings.HasPrefix(source, "s3://") {
		parts := strings.SplitN(source[5:], "/", 2)
		if len(parts) < 2 {
			fmt.Println("Invalid source path format")
			return
		}
		bucketName := parts[0]
		objectPrefix := parts[1]

		if strings.HasPrefix(destination, "s3://") {
			// Copy between S3 buckets (not implemented)
			fmt.Println("S3-to-S3 copy not implemented yet")
			return
		} else {
			// Download from S3 to local
			err := downloadFromMinio(ctx, MinioClient, bucketName, objectPrefix, destination, recursive)
			if err != nil {
				fmt.Println("Error downloading from MinIO:", err)
			}
		}
	} else if strings.HasPrefix(destination, "s3://") {
		parts := strings.SplitN(destination[5:], "/", 2)
		if len(parts) < 2 {
			fmt.Println("Invalid destination path format")
			return
		}
		bucketName := parts[0]
		objectPrefix := parts[1]

		// Upload from local to S3
		err := uploadToMinio(ctx, MinioClient, source, bucketName, objectPrefix, recursive)
		if err != nil {
			fmt.Println("Error uploading to MinIO:", err)
		}
	} else {
		// Local to local copy is not handled
		fmt.Println("Local-to-local copy not implemented")
	}
}

// Download a file or directory from MinIO
func downloadFromMinio(ctx context.Context, MinioClient *minio.Client, bucketName string, objectPrefix string, localPath string, recursive bool) error {

	if recursive {
		objectCh := MinioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
			Prefix:    objectPrefix,
			Recursive: true,
		})

		for object := range objectCh {
			if object.Err != nil {
				return fmt.Errorf("error listing objects: %w", object.Err)
			}

			localFilePath := filepath.Join(localPath, strings.TrimPrefix(object.Key, objectPrefix+"/"))
			if err := os.MkdirAll(filepath.Dir(localFilePath), os.ModePerm); err != nil {
				return err
			}

			file, err := os.Create(localFilePath)
			if err != nil {
				return err
			}
			defer file.Close()

			objectReader, err := MinioClient.GetObject(ctx, bucketName, object.Key, minio.GetObjectOptions{})
			if err != nil {
				return err
			}
			defer objectReader.Close()

			_, err = io.Copy(file, objectReader)
			if err != nil {
				return err
			}
		}
	} else {
		objectReader, err := MinioClient.GetObject(ctx, bucketName, objectPrefix, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		defer objectReader.Close()

		localFile, err := os.Create(localPath)
		if err != nil {
			return err
		}
		defer localFile.Close()

		_, err = io.Copy(localFile, objectReader)
		if err != nil {
			return err
		}
	}

	return nil
}

// Upload a local file or directory to MinIO
func uploadToMinio(ctx context.Context, MinioClient *minio.Client, localPath string, bucketName string, objectPrefix string, recursive bool) error {

	// Handle directory upload
	if recursive {
		err := filepath.WalkDir(localPath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("error accessing path %q: %w", path, err)
			}

			relPath, _ := filepath.Rel(localPath, path)
			objectName := filepath.Join(objectPrefix, relPath)

			if info.IsDir() {
				if !recursive {
					return filepath.SkipDir
				}
				return nil
			}

			// Upload the file
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %q: %w", path, err)
			}
			defer file.Close()

			_, err = MinioClient.PutObject(ctx, bucketName, objectName, file, -1, minio.PutObjectOptions{})
			if err != nil {
				return fmt.Errorf("failed to upload file %q: %w", path, err)
			}

			fmt.Printf("Successfully uploaded %q to %q\n", path, objectName)
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}

	} else {
		// Handle single file upload
		file, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = MinioClient.PutObject(ctx, bucketName, objectPrefix, file, -1, minio.PutObjectOptions{})
		return err
	}

	return nil
}
