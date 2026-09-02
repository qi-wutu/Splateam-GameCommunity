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
	// 校验 JWT 密钥：空 / 默认值 / 过短时直接拒绝启动，避免带病运行
	if err := config.ValidateJWT(); err != nil {
		panic(err)
	}
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
