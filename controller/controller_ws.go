package controller

import (
	"log"
	"net/http"

	"splatoon-backend/service"
	"splatoon-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 开发阶段允许所有来源
	},
}

func WebSocketHandler(ctx *gin.Context) {
	// 从 query 取 token 并验证
	token := ctx.Query("token")
	userID, err := utils.ParseJWT(token)
	if err != nil {
		ctx.JSON(401, gin.H{"error": "invalid token"})
		return
	}

	//升级 HTTP → WebSocket
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	//创建 Client → 注册到 Hub
	client := service.NewClient(service.GlobalHub, conn, userID)
	service.GlobalHub.Register <- client

	// 启动读写 goroutine
	go client.WritePump()
	go client.ReadPump()
}
