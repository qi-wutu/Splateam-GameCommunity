package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		fmt.Println("⚠️ 未找到 .env 文件，使用默认配置")
	}

	// 从环境变量读取配置
	dbHost := getEnv("DB_HOST", "127.0.0.1")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "root")
	dbPass := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "splatoon")
	serverPort := getEnv("SERVER_PORT", "8080")

	// 连接 MySQL,这里dsn直接从env中读取后拼接
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("数据库连接失败：" + err.Error())
	}
	fmt.Println("✅ MySQL 连接成功")

	// 自动建表（后续加 models）
	// db.AutoMigrate(&models.User{}, &models.Party{})

	// Gin 路由
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// 健康检查（包含数据库状态）
	r.GET("/health", func(c *gin.Context) {
		sqlDB, _ := db.DB()
		err := sqlDB.Ping()
		if err != nil {
			c.JSON(500, gin.H{"status": "unhealthy", "db": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "db": "connected"})
	})

	// 启动
	fmt.Printf("🚀 服务启动在 :%s\n", serverPort)
	r.Run(":" + serverPort)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
