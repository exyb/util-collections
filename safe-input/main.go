package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	. "github.com/exyb/safe-input/client"
	. "github.com/exyb/safe-input/qrcode"
)

func main() {
	// 解析命令行参数
	username := flag.String("u", "", "Username for authentication")
	isValidate := flag.Bool("c", false, "validate qr code for authentication")
	clientType := flag.String("t", "", "client type")

	//管理参数
	isGenerate := flag.Bool("g", false, "generate qr code for authentication")
	adminPass := flag.String("p", "", "password to generate authentication")
	saveQRCodeToFile := flag.Bool("s", false, "save qr code to file")

	// 自定义 Usage 函数
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		// 只打印我们想要显示的标志
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name != "g" && f.Name != "p" && f.Name != "s" {
				fmt.Fprintf(os.Stderr, "  -%s: %s (default: %v)\n", f.Name, f.Usage, f.DefValue)
			}
		})
		help := `
Supported Commands:
  minio: support conn/ls/mb/rb/cp/rm/rmr/find/tree/cat/head/mv
  redis: support keys/get/set/scan/dbsize/presist/del/type/ttl/expire/ttl/setnx/setex/mset/mget/lpush/rpush/append
  mysql: support all valid MySQL commands

`
		fmt.Println(help)

	}

	// 解析标志
	flag.Parse()

	if *isGenerate {
		if *adminPass == "" {
			return
		}
		if *adminPass != "zhfRd#999" {
			return
		}
		GeneratingQRCode(*username, *saveQRCodeToFile)
		return
	}
	if *username == "" {
		fmt.Println("Error: Username (-u) is required.")
		os.Exit(1)
	}
	if *clientType == "" && !(*isGenerate || *isValidate) {
		fmt.Println("Error: ClientType (-t) is required.")
		os.Exit(1)
	}

	// 提示用户输入 OTP 密码
	fmt.Print("Enter OTP password: ")
	reader := bufio.NewReader(os.Stdin)
	otpPassword, _ := reader.ReadString('\n')
	otpPassword = otpPassword[:len(otpPassword)-1] // 去掉换行符

	valid := ValidateOTP(otpPassword, *username)
	if !valid {
		fmt.Println("Error: Invalid OTP.")
		os.Exit(1)
	}
	if *isValidate {
		return
	}

	// 从环境变量中读取密码
	redisHost := GetEnv("SPRING_REDIS_HOST", "localhost")
	redisPort := GetEnv("SPRING_REDIS_PORT", "6379")
	redisDb, _ := strconv.Atoi(GetEnv("SPRING_REDIS_DB", "0"))

	mysqlHost := GetEnv("MPC_DB_HOST", "127.0.0.1")
	mysqlPort := GetEnv("MPC_DB_PORT", "3306")
	mysqlUser := GetEnv("MPC_DB_USER", "mpc")
	mysqlSchema := GetEnv("MPC_DB_SCHEMA", "mpc_runtime")

	minioAddreess := GetEnv("MPC_OSS_ADDRESS", "http://localhost:9000")
	minioAccessKey := GetEnv("MPC_OSS_ACCESS_KEY", "minioadmin")
	minioSecretKey := GetEnv("MPC_OSS_SECRET_KEY", "minioadmin")

	switch strings.ToLower(*clientType) {
	case "redis":
		password := os.Getenv("SPRING_REDIS_PASSWORD")
		StartRedisClient(password, redisHost, redisPort, redisDb)
	case "minio":
		StartMinioClient(minioAddreess, minioAccessKey, minioSecretKey)
	case "mysql":
		password := GetEnv("MPC_DB_PASSWORD", "m.p.c!@Fr02oee")

		if password == "" {
			fmt.Println("Error: MYSQL_PASSWORD environment variable is not set.")
			os.Exit(1)
		}
		StartMySQLClient(password, mysqlUser, mysqlHost, mysqlPort, mysqlSchema)
	case "mysqlroot":
		password := os.Getenv("MYSQL_ROOT_PASSWORD")
		if password == "" {
			fmt.Println("Error: MYSQL_ROOT_PASSWORD environment variable is not set.")
			os.Exit(1)
		}
		StartMySQLClient(password, "root", mysqlHost, mysqlPort, mysqlSchema)
	default:
		fmt.Println("Invalid client name. Please choose from redis, mysql, mysqlroot, minio.")
		os.Exit(1)
	}

}

func GetEnv(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}
