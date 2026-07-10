package router

import (
	"splatoon-backend/controller"
	"splatoon-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	// Gin 路由
	r := gin.Default()
	// 公开路由
	auth := r.Group("/api/auth")
	{
		// 用户注册登录
		auth.POST("/register", controller.RegisterUser)
		auth.POST("/login", controller.Login)
	}

	// 受保护路由
	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		// 后续功能放这里
	}

	return r
}
