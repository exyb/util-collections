package client

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	actions "github.com/exyb/safe-input/client/cmd"
	minioCmd "github.com/exyb/safe-input/client/cmd/minio"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/peterh/liner"
	"golang.org/x/net/context"
)

var MinioClient *minio.Client
var cancelFunc context.CancelFunc

// Initialize MinIO client
func setupMinio(endpoint, accessKeyID, secretAccessKey string) {
	useHTTPS := strings.HasPrefix(endpoint, "https://")

	// Strip the protocol from the endpoint
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useHTTPS,
	})
	if err != nil {
		log.Fatalln("Failed to create MinIO client:", err)
	}
}

// Main interactive function
func interactiveShell() {
	line := liner.NewLiner()
	defer line.Close()
	// 加载历史记录
	if f, err := os.Open(".minioCmd.history"); err == nil {
		line.ReadHistory(f)
		f.Close()
	}
	// Enable command history
	line.SetCtrlCAborts(true)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	for {
		select {
		case <-sigChan:
			if cancelFunc != nil {
				cancelFunc() // Cancel the current command
				fmt.Println("\nCommand cancelled")
				continue
			}
			// minioCmd.Handle the case where no command is running
			fmt.Println("\nNo command running to cancel")
			continue
		default:
			cmd, err := line.Prompt("minio> ")
			if err != nil || err == io.EOF {
				log.Print("Exiting...")
				break
			}

			line.AppendHistory(cmd)

			// minioCmd.Handle commands
			cmd = strings.TrimSpace(cmd)
			cmd = strings.Replace(cmd, "  ", " ", -1)
			args := strings.Split(cmd, " ")
			lastAction := &actions.LastAction{Action: args[0]}
			switch args[0] {
			case "conn":
				if len(args) < 4 {
					fmt.Println("Usage: conn <endpoint> <accessKey> <secretKey>")
					continue
				}
				setupMinio(args[1], args[2], args[3])
			case "ls":
				if len(args) < 2 {
					minioCmd.HandleLs(ctx, MinioClient, "")
					continue
				} else if len(args) == 2 {
					minioCmd.HandleLs(ctx, MinioClient, args[1])
				} else {
					fmt.Println("Usage: ls [bucket/folder] or ls /")
					fmt.Println("Tips: add '/' to the end of the path to list all objects in the folder")
				}
			case "exit":
				fmt.Println("Exiting...")
				return
			case "mkdir":
				if len(args) < 2 {
					fmt.Println("Usage: mkdir [-r] <bucket>/<directory>")
				} else {
					minioCmd.HandleMkdirCommand(ctx, MinioClient, args[1], len(args) > 2 && args[2] == "-r")
				}
			case "cp":
				if len(args) > 1 {
					minioCmd.HandleCpCommand(ctx, MinioClient, args[1:])
					continue
				} else {
					fmt.Println("Usage: cp [-r] <source> <destination>")
					fmt.Println("Tips: add s3:// as the prefix of remote minio object")
				}
			case "mv":
				if len(args) != 3 {
					fmt.Println("Usage: mv <src_bucket>/<src_object> <dest_bucket>/<dest_object>")
					return
				}
				srcBucketName := strings.Split(args[1], "/")[0]
				srcObjectName := strings.Join(strings.Split(args[1], "/")[1:], "/")
				objBucketName := strings.Split(args[1], "/")[0]
				objObjectName := strings.Join(strings.Split(args[1], "/")[1:], "/")

				minioCmd.HandleMvCommand(ctx, MinioClient, srcBucketName, srcObjectName, objBucketName, objObjectName)

			case "head":
				if len(args) < 2 {
					fmt.Println("Usage: head [-lines] <bucket-name> ")
					continue
				}
				lineCount := 10 // 默认显示前10行
				if len(args) == 3 {
					var err error
					lineNo := strings.Replace(args[1], "-", "", 1)
					args[1] = ""
					lineCount, err = strconv.Atoi(lineNo)
					if err != nil {
						fmt.Println("Invalid line count:", err)
						continue
					}
				}
				bucketObjectName := args[len(args)-1]
				bucketName := strings.Split(bucketObjectName, "/")[0]
				objectName := strings.Join(strings.Split(bucketObjectName, "/")[1:], "/")

				minioCmd.HandleHeadCommand(ctx, MinioClient, bucketName, objectName, lineCount)
			case "cat":
				if len(args) != 2 {
					fmt.Println("Usage: cat <bucket-name>")
					continue
				}
				bucketName := strings.Split(args[1], "/")[0]
				objectName := strings.Join(strings.Split(args[1], "/")[1:], "/")
				minioCmd.HandleCatCommand(ctx, MinioClient, bucketName, objectName)
			case "rb":
				if len(args) != 2 {
					fmt.Println("Usage: rb <bucket>")
					continue
				}
				bucketName := args[1]
				minioCmd.HandleRbCommand(ctx, MinioClient, bucketName)
			case "tree":
				if len(args) < 2 {
					fmt.Println("Usage: tree <bucket>/[prefix]")
					continue
				}

				bucketName := strings.Split(args[1], "/")[0]
				objectName := strings.Join(strings.Split(args[1], "/")[1:], "/")

				minioCmd.HandleTreeCommand(ctx, MinioClient, bucketName, objectName)
			case "undo":
				minioCmd.HandleUndoCommand(ctx, MinioClient, lastAction)
			case "mb":
				if len(args) != 2 {
					fmt.Println("Usage: mb <bucket> ")
					return
				}
				bucketName := args[1]
				minioCmd.HandleMbCommand(ctx, MinioClient, bucketName)
			case "find":
				if len(args) != 3 {
					fmt.Println("Usage: find <bucket> <pattern>")
					return
				}
				minioCmd.HandleFindCommand(ctx, MinioClient, args[1], args[2])
			case "rm":
				if len(args) < 2 {
					fmt.Println("Usage: rm <bucket-object>")
					return
				}
				bucketName := strings.Split(args[1], "/")[0]
				filePrefix := strings.Join(strings.Split(args[1], "/")[1:], "/")
				minioCmd.HandleRmCommand(ctx, MinioClient, bucketName, filePrefix, false, false)
			case "rmr":
				if len(args) < 2 {
					fmt.Println("Usage: rmr <bucket-object> [--force]")
					return
				}
				force := len(args) == 3 && args[2] == "--force"
				if args[len(args)-1] == "--force" {
					args[len(args)-1] = ""
				}
				bucketName := strings.Split(args[1], "/")[0]
				filePrefix := strings.Join(strings.Split(args[1], "/")[1:], "/")

				minioCmd.HandleRmCommand(ctx, MinioClient, bucketName, filePrefix, true, force)
			default:
				fmt.Println("Unknown command:", args[0])
			}
		}
		// 保存历史记录到文件
		if f, err := os.Create(".minioCmd.history"); err != nil {
			fmt.Println("保存历史记录失败:", err)
		} else {
			line.WriteHistory(f)
			f.Close()
		}
	}

}

func StartMinioClient(endpoint, accessKeyID, secretAccessKey string) {

	setupMinio(endpoint, accessKeyID, secretAccessKey)
	fmt.Println("MinIO interactive shell. Type 'exit' to quit.")
	interactiveShell()
}
