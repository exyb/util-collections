package client

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/peterh/liner"
)

func StartMySQLClient(password, user, host, port, schema string) {

	if user != "" {
		user = "mpc"
	}
	if host != "" {
		host = "localhost"
	}
	if port != "" {
		port = "3306"
	}
	if schema != "" {
		schema = "mpc_runtime"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, schema)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Could not connect to MySQL: %v\n", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Could not ping MySQL: %v\n", err)
	}
	fmt.Println("Connected to MySQL!")

	interactiveMySQL(db)
}

func interactiveMySQL(db *sql.DB) {
	fmt.Println("Entering MySQL interactive mode. Type 'exit' to quit.")
	// 创建一个新的 line reader
	line := liner.NewLiner()
	defer line.Close()

	// 设置多个选项，比如自动补全和历史记录
	line.SetCtrlCAborts(true)

	// 加载历史记录
	if f, err := os.Open(".mysql.history"); err == nil {
		line.ReadHistory(f)
		f.Close()
	}

	for {
		query, err := line.Prompt("mysql> ")

		// 如果用户按 Ctrl+C 或者输入空行，则退出
		if err != nil {
			fmt.Println("退出程序")
			break
		}

		// 存储输入到历史中，避免重复空命令
		if query != "" {
			line.AppendHistory(query)
		}

		// 检查用户输入是否是某些特殊命令，比如 'exit' 或 'quit'
		if query == "exit" || query == "quit" {
			fmt.Println("退出程序")
			break
		}
		// 处理退出命令
		if strings.ToLower(query) == "exit" {
			fmt.Println("退出 mysql 交互模式。")
			break
		}

		rows, err := db.Query(query)
		if err != nil {
			fmt.Println("Query Error:", err)
			continue
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			fmt.Println("Error getting columns:", err)
			continue
		}

		// 打印表头
		if len(columns) > 0 {
			fmt.Println("| rowNum | " + strings.Join(columns, " | ") + " |")
			// 打印分隔行
			fmt.Print("| -- | ")
			colCount := len(columns)

			for i := 0; i < colCount; i++ {
				fmt.Print("-- |")
			}
			fmt.Println("")
		}

		// 准备存储行数据的切片
		vals := make([]interface{}, len(columns))
		scanArgs := make([]interface{}, len(columns))
		rowNum := 0 // 行号计数器

		for rows.Next() {
			// 需要为每列分配合适的类型
			for i := range vals {
				scanArgs[i] = &sql.NullString{}
			}

			// 扫描行
			if err := rows.Scan(scanArgs...); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}

			fmt.Printf("| %d | ", rowNum+1)
			// 打印行数据
			for i, v := range scanArgs {
				// 根据类型进行类型断言
				if ns, ok := v.(*sql.NullString); ok {
					if ns.Valid {
						fmt.Printf("%s ", ns.String)
					} else {
						fmt.Printf("NULL ")
					}
				} else if ni, ok := v.(*sql.NullInt64); ok {
					if ni.Valid {
						fmt.Printf("%d ", ni.Int64)
					} else {
						fmt.Printf("NULL ")
					}
				} else if nf, ok := v.(*sql.NullFloat64); ok {
					if nf.Valid {
						fmt.Printf("%f ", nf.Float64)
					} else {
						fmt.Printf("NULL ")
					}
				} else {
					fmt.Printf("UNKNOWN ")
				}
				// 打印列分隔符
				if i < len(scanArgs)-1 {
					fmt.Print(" | ")
				}
			}
			fmt.Println(" |")

			rowNum++ // 增加行号计数器

		}
	}

	// 保存历史记录到文件
	if f, err := os.Create(".mysql.history"); err != nil {
		fmt.Println("保存历史记录失败:", err)
	} else {
		line.WriteHistory(f)
		f.Close()
	}
}
