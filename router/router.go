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
		auth.POST("/register", controller.RegisterUser)
		auth.POST("/login", controller.Login)
	}
	r.GET("/api/party", controller.PartyList)

	// 受保护路由
	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		api.POST("/party", controller.CreateParty)
		api.DELETE("/party/:id", controller.DeleteParty)
		api.POST("/party/:id/join", controller.JoinParty)
		api.POST("/party/:id/leave", controller.LeaveParty)
	}

	return r
}
