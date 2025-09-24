// 文件中转站 - 单文件实现
// 使用 gin 框架，支持文件上传、下载与缓存自动过期
package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 缓存项结构体
type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

// 全局缓存
var (
	cache = make(map[string]*cacheItem)
	mu    sync.RWMutex
)

// 解析 keepDuration 字符串，返回 time.Duration，支持 d/h/m，默认3m
func parseDuration(s string) time.Duration {
	if s == "" {
		return 3 * time.Minute
	}
	last := s[len(s)-1]
	num := s[:len(s)-1]
	var n int
	var err error
	if last == 'd' {
		n, err = strconv.Atoi(num)
		if err == nil {
			return time.Duration(n) * 24 * time.Hour
		}
	} else if last == 'h' {
		n, err = strconv.Atoi(num)
		if err == nil {
			return time.Duration(n) * time.Hour
		}
	} else if last == 'm' {
		n, err = strconv.Atoi(num)
		if err == nil {
			return time.Duration(n) * time.Minute
		}
	}
	// 解析失败，返回默认3分钟
	return 3 * time.Minute
}

// 定时清理过期缓存
func startCleaner() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			now := time.Now()
			mu.Lock()
			for k, v := range cache {
				if now.After(v.expiresAt) {
					delete(cache, k)
				}
			}
			mu.Unlock()
		}
	}()
}

func main() {
	// 解析端口参数，默认8080
	port := "8080"
	if len(os.Args) > 1 {
		for i, arg := range os.Args {
			if (arg == "--port" || arg == "-p") && i+1 < len(os.Args) {
				port = os.Args[i+1]
			}
		}
	}

	r := gin.Default()

	// 上传接口
	r.POST("/*path", func(c *gin.Context) {
		// 解析 path 和 keepDuration
		parts := strings.Split(strings.TrimLeft(c.Param("path"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			c.String(http.StatusBadRequest, "路径不能为空")
			return
		}
		path := parts[0]
		keep := ""
		if len(parts) > 1 {
			keep = parts[1]
		}
		duration := parseDuration(keep)

		// 读取 body
		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, "读取文件失败")
			return
		}
		// 存入缓存
		mu.Lock()
		cache[path] = &cacheItem{
			data:      data,
			expiresAt: time.Now().Add(duration),
		}
		mu.Unlock()
		c.String(http.StatusOK, "上传成功，路径: %s，保存时长: %s", path, duration.String())
	})

	// 下载接口
	r.GET("/*path", func(c *gin.Context) {
		path := strings.TrimLeft(c.Param("path"), "/")
		if path == "" {
			c.String(http.StatusBadRequest, "路径不能为空")
			return
		}
		mu.RLock()
		item, ok := cache[path]
		mu.RUnlock()
		if !ok || time.Now().After(item.expiresAt) {
			c.String(http.StatusNotFound, "文件不存在或已过期")
			return
		}
		// 返回文件内容
		reader := bytes.NewReader(item.data)
		c.Header("Content-Disposition", "attachment; filename="+path)
		c.DataFromReader(http.StatusOK, int64(len(item.data)), "application/octet-stream", reader, nil)
	})

	startCleaner()

	// 启动服务，监听指定端口
	r.Run("0.0.0.0:" + port)
}
