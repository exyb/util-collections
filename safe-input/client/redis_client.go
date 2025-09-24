package client

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/peterh/liner"
	"golang.org/x/net/context"
)

var ctx = context.Background()

func StartRedisClient(password, host, port string, db int) {
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "6379"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v\n", err)
	}
	fmt.Println("Connected to Redis!")

	interactiveRedis(rdb)
}

func interactiveRedis(rdb *redis.Client) {
	fmt.Println("Entering Redis interactive mode. Type 'exit' to quit.")

	// 创建一个新的 line reader
	line := liner.NewLiner()
	defer line.Close()

	// 设置多个选项，比如自动补全和历史记录
	line.SetCtrlCAborts(true)

	// 加载历史记录
	if f, err := os.Open(".redis.history"); err == nil {
		line.ReadHistory(f)
		f.Close()
	}

	// reader := bufio.NewReader(os.Stdin)

	for {

		/* fmt.Print("redis> ")
		input, err := reader.ReadString('\n')
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			log.Printf("读取输入错误: %v", err)
			continue
		}
		if err != nil {
			log.Printf("读取输入错误: %v", err)
			continue
		}
		*/
		input, err := line.Prompt("redis> ")

		// 如果用户按 Ctrl+C 或者输入空行，则退出
		if err != nil {
			fmt.Println("退出程序")
			break
		}

		// 存储输入到历史中，避免重复空命令
		if input != "" {
			line.AppendHistory(input)
		}

		// 检查用户输入是否是某些特殊命令，比如 'exit' 或 'quit'
		if input == "exit" || input == "quit" {
			fmt.Println("退出程序")
			break
		}
		// 处理退出命令
		if strings.ToLower(input) == "exit" {
			fmt.Println("退出 Redis 交互模式。")
			break
		}

		// 解析并执行命令
		commandParts := strings.Fields(input)
		if len(commandParts) == 0 {
			continue
		}

		command := strings.ToUpper(commandParts[0])
		switch command {
		case "SET":
			if len(commandParts) != 3 {
				fmt.Println("使用方法: SET <key> <value>")
				continue
			}
			err := rdb.Set(ctx, commandParts[1], commandParts[2], 0).Err()
			if err != nil {
				log.Printf("SET 错误: %v", err)
			} else {
				fmt.Println("OK")
			}
		case "SCAN":
			if len(commandParts) > 5 { // SCAN 命令最多允许 5 个部分 (cursor, MATCH, pattern, COUNT, count)
				fmt.Println("使用方法: SCAN <cursor> [MATCH <pattern>] [COUNT <count>]")
				continue
			}

			cursor := uint64(0) // 默认游标
			match := ""
			count := int64(0) // 默认数量

			for i := 1; i < len(commandParts); i++ {
				switch strings.ToUpper(commandParts[i]) {
				case "MATCH":
					if i+1 >= len(commandParts) {
						fmt.Println("MATCH 选项后必须跟模式")
						continue
					}
					match = commandParts[i+1]
					i++ // 跳过模式
				case "COUNT":
					if i+1 >= len(commandParts) {
						fmt.Println("COUNT 选项后必须跟数量")
						continue
					}
					var err error
					count, err = strconv.ParseInt(commandParts[i+1], 10, 64)
					if err != nil {
						fmt.Println("COUNT 数量无效")
						continue
					}
					i++ // 跳过数量
				default:
					var err error
					cursor, err = strconv.ParseUint(commandParts[i], 10, 64)
					if err != nil {
						fmt.Println("无效的游标值")
						continue
					}
				}
			}

			// 执行 SCAN 命令
			for {
				// 获取 SCAN 的结果
				// func (cmd *redis.ScanCmd) Result() (keys []string, cursor uint64, err error)

				keys, newCursor, err := rdb.Scan(ctx, cursor, match, count).Result()
				if err != nil {
					log.Printf("SCAN 错误: %v", err)
					break
				}

				// 打印匹配的键
				fmt.Println("匹配的键:", keys)

				// 更新游标
				cursor = newCursor

				// 如果游标为 0，表示扫描结束
				if cursor == 0 {
					break
				}
			}

			var intCursor int
			if cursor <= math.MaxInt {
				intCursor = int(cursor)
				fmt.Printf("下一个游标: %s\n", strconv.Itoa(intCursor)) // 打印下一个游标
			} else {
				// 如果 cursor 值超出了 int 的范围，则直接转换为字符串
				fmt.Printf("下一个游标: %s\n", strconv.FormatUint(cursor, 10)) // 打印下一个游标
			}

		case "GET":
			if len(commandParts) != 2 {
				fmt.Println("使用方法: GET <key>")
				continue
			}
			val, err := rdb.Get(ctx, commandParts[1]).Result()
			if err != nil {
				log.Printf("GET 错误: %v", err)
			} else {
				fmt.Println(val)
			}

		case "KEYS":
			if len(commandParts) != 2 {
				fmt.Println("使用方法: KEYS <pattern>")
				continue
			}
			keys, err := rdb.Keys(ctx, commandParts[1]).Result()
			if err != nil {
				log.Printf("KEYS 错误: %v", err)
			} else {
				fmt.Println("匹配的键:", keys)
			}

		case "DBSIZE":
			size, err := rdb.DBSize(ctx).Result()
			if err != nil {
				log.Printf("DBSIZE 错误: %v", err)
			} else {
				fmt.Printf("数据库中键的数量: %d\n", size)
			}

		case "PERSIST":
			if len(commandParts) != 2 {
				fmt.Println("使用方法: PERSIST <key>")
				continue
			}
			result, err := rdb.Persist(ctx, commandParts[1]).Result()
			if err != nil {
				log.Printf("PERSIST 错误: %v", err)
			} else {
				fmt.Println("键的持久化:", result)
			}

		case "DEL":
			if len(commandParts) != 2 {
				fmt.Println("使用方法: DEL <key>")
				continue
			}
			err := rdb.Del(ctx, commandParts[1]).Err()
			if err != nil {
				log.Printf("DEL 错误: %v", err)
			} else {
				fmt.Println("OK")
			}

		case "TYPE":
			if len(commandParts) != 2 {
				fmt.Println("使用方法: TYPE <key>")
				continue
			}
			keyType, err := rdb.Type(ctx, commandParts[1]).Result()
			if err != nil {
				log.Printf("TYPE 错误: %v", err)
			} else {
				fmt.Println("类型:", keyType)
			}

		case "TTL":
			if len(commandParts) != 2 {
				fmt.Println("使用方法: TTL <key>")
				continue
			}
			ttl, err := rdb.TTL(ctx, commandParts[1]).Result()
			if err != nil {
				log.Printf("TTL 错误: %v", err)
			} else {
				fmt.Println("TTL:", ttl)
			}

		case "EXPIRE":
			if len(commandParts) != 3 {
				fmt.Println("使用方法: EXPIRE <key> <seconds>")
				continue
			}
			seconds, err := strconv.Atoi(commandParts[2])
			if err != nil {
				fmt.Println("秒数必须是一个整数")
				continue
			}
			err = rdb.Expire(ctx, commandParts[1], time.Duration(seconds)*time.Second).Err()
			if err != nil {
				log.Printf("EXPIRE 错误: %v", err)
			} else {
				fmt.Println("OK")
			}
		case "SETNX":
			if len(commandParts) != 3 {
				fmt.Println("使用方法: SETNX <key> <value>")
				continue
			}
			success, err := rdb.SetNX(ctx, commandParts[1], commandParts[2], 0).Result()
			if err != nil {
				log.Printf("SETNX 错误: %v", err)
			} else {
				fmt.Println("设置成功:", success)
			}

		case "SETEX":
			if len(commandParts) != 4 {
				fmt.Println("使用方法: SETEX <key> <seconds> <value>")
				continue
			}
			seconds, err := strconv.Atoi(commandParts[2])
			if err != nil {
				fmt.Println("秒数必须是一个整数")
				continue
			}
			err = rdb.SetEX(ctx, commandParts[1], commandParts[3], time.Duration(seconds)*time.Second).Err()
			if err != nil {
				log.Printf("SETEX 错误: %v", err)
			} else {
				fmt.Println("OK")
			}

		case "MSET":
			if len(commandParts) < 3 || len(commandParts)%2 == 0 {
				fmt.Println("使用方法: MSET <key1> <value1> [<key2> <value2> ...]")
				continue
			}
			// 将键值对加入 map
			msetArgs := make(map[string]interface{})
			for i := 1; i < len(commandParts); i += 2 {
				msetArgs[commandParts[i]] = commandParts[i+1]
			}
			err = rdb.MSet(ctx, msetArgs).Err()
			if err != nil {
				log.Printf("MSET 错误: %v", err)
			} else {
				fmt.Println("OK")
			}

		case "MGET":
			if len(commandParts) < 2 {
				fmt.Println("使用方法: MGET <key1> [<key2> ...]")
				continue
			}
			keys := commandParts[1:] // 获取多重键
			values, err := rdb.MGet(ctx, keys...).Result()
			if err != nil {
				log.Printf("MGET 错误: %v", err)
			} else {
				fmt.Println(values)
			}

		case "APPEND":
			if len(commandParts) != 3 {
				fmt.Println("使用方法: APPEND <key> <value>")
				continue
			}
			newSize, err := rdb.Append(ctx, commandParts[1], commandParts[2]).Result()
			if err != nil {
				log.Printf("APPEND 错误: %v", err)
			} else {
				fmt.Printf("新大小: %d\n", newSize)
			}

		case "LPUSH":
			if len(commandParts) < 3 {
				fmt.Println("使用方法: LPUSH <key> <value1> [<value2> ...]")
				continue
			}
			key := commandParts[1]
			values := commandParts[2:] // 获取所有要插入的值
			count, err := rdb.LPush(ctx, key, values).Result()
			if err != nil {
				log.Printf("LPUSH 错误: %v", err)
			} else {
				fmt.Printf("新列表长度: %d\n", count)
			}

		case "RPUSH":
			if len(commandParts) < 3 {
				fmt.Println("使用方法: RPUSH <key> <value1> [<value2> ...]")
				continue
			}
			key := commandParts[1]
			values := commandParts[2:] // 获取所有要插入的值
			count, err := rdb.RPush(ctx, key, values).Result()
			if err != nil {
				log.Printf("RPUSH 错误: %v", err)
			} else {
				fmt.Printf("新列表长度: %d\n", count)
			}

		default:
			fmt.Println("暂时不支持命令: ", command)
		}
	}

	// 保存历史记录到文件
	if f, err := os.Create(".redis.history"); err != nil {
		fmt.Println("保存历史记录失败:", err)
	} else {
		line.WriteHistory(f)
		f.Close()
	}
}
