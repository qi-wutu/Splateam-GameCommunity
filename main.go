package main

import (
	"fmt"
	"splatoon-backend/config"
	"splatoon-backend/router"
)

func main() {
	// 加载 .env 文件
	config.LoadEnv()
	//数据库初始化
	config.InitMySQL()
	// Gin 路由
	router.SetupRouter()

	fmt.Printf("🚀 服务启动在 :%s\n", config.ServerPort)
	r := router.SetupRouter()
	r.Run(":" + config.ServerPort)
}
