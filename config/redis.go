package config

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
	RedisCtx    = context.Background()
)

// InitRedis 初始化 Redis 连接
// 允许连接失败（非致命错误），RedisClient 为 nil 时所有调用自动跳过
func InitRedis() {
	addr := GetEnv("REDIS_ADDR", "127.0.0.1:6379")
	password := GetEnv("REDIS_PASSWORD", "")

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0, // 默认 0 号数据库
	})

	_, err := RedisClient.Ping(RedisCtx).Result()
	if err != nil {
		fmt.Printf("⚠️  Redis 连接失败（非致命，继续启动）: %v\n", err)
		fmt.Println("   Redis 功能（在线状态 / 消息缓存）将不可用")
		RedisClient = nil
	} else {
		fmt.Println("✅ Redis 连接成功")
	}
}
