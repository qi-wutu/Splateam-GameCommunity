package router

import (
	"splatoon-backend/config"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	// Gin 路由
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// 健康检查（包含数据库状态）
	r.GET("/health", func(c *gin.Context) {
		sqlDB, _ := config.Db.DB()
		err := sqlDB.Ping()
		if err != nil {
			c.JSON(500, gin.H{"status": "unhealthy", "db": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "db": "connected"})
	})
	return r
}
