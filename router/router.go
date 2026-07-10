package router

import (
	"splatoon-backend/config"
	"splatoon-backend/controller"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	// Gin 路由
	r := gin.Default()
	User := r.Group("/user")
	{
		// 用户注册登录
		User.POST("/register", controller.RegisterUser)
		User.POST("/login", controller.Login)
	}

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
