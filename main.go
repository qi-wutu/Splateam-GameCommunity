package main

import (
	"fmt"
	"splatoon-backend/config"
	"splatoon-backend/router"
	"splatoon-backend/service"
)

func main() {
	// 加载 .env 文件
	config.LoadEnv()
	//数据库初始化
	config.InitMySQL()
	config.InitRedis()
	config.InitRabbitMQ()
	// 启动 WebSocket Hub
	go service.GlobalHub.Run()
	// 启动邮件 Worker（异步消费 MQ）
	go service.StartMailWorker()
	fmt.Printf("🚀 服务启动在 :%s\n", config.ServerPort)
	r := router.SetupRouter()
	r.Run(":" + config.ServerPort)
}
