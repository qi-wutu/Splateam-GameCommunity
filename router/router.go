package router

import (
	"splatoon-backend/controller"
	"splatoon-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	// Gin 路由
	r := gin.Default()
	r.Use(middlewares.CORS())
	// 公开路由
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", controller.RegisterUser)
		auth.POST("/login", controller.Login)
		auth.POST("/activate", controller.ActivateAccount) // 邮箱激活
	}
	r.GET("/api/party", controller.PartyList)
	r.GET("/api/party/:id", controller.GetParty)
	// WebSocket（公开路由，handler 内自行验证 JWT）
	r.GET("/api/ws", controller.WebSocketHandler)

	// 受保护路由
	api := r.Group("/api")
	api.Use(middlewares.AuthMiddleware())
	{
		api.GET("/user/me", controller.GetCurrentUser)
		api.GET("/user/:id/online", controller.GetUserOnline)
		api.POST("/party", controller.CreateParty)
		api.DELETE("/party/:id", controller.DeleteParty)
		api.POST("/party/:id/join", controller.JoinParty)
		api.POST("/party/:id/leave", controller.LeaveParty)
	}

	return r
}
