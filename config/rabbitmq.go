package config

import (
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	RabbitMQConn *amqp.Connection
	RabbitMQCh   *amqp.Channel
)

// InitRabbitMQ 初始化 RabbitMQ 连接
// 失败时优雅降级，不阻塞启动
func InitRabbitMQ() {
	url := GetEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")

	conn, err := amqp.Dial(url)
	if err != nil {
		fmt.Printf("⚠️  RabbitMQ 连接失败（非致命，继续启动）: %v\n", err)
		fmt.Println("   邮件发送功能将不可用")
		return
	}

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("⚠️  RabbitMQ Channel 创建失败: %v\n", err)
		conn.Close()
		return
	}

	// 声明交换机（topic 类型，支持路由）
	err = ch.ExchangeDeclare(
		"splatoon.events", // 名称
		"topic",           // 类型
		true,              // 持久化
		false,             // 自动删除
		false,             // 内部
		false,             // 不等待
		nil,
	)
	if err != nil {
		fmt.Printf("⚠️  交换机声明失败: %v\n", err)
		conn.Close()
		return
	}

	// 声明队列（持久化，重启后队列还在）
	_, err = ch.QueueDeclare(
		"mail.welcome", // 名称
		true,           // 持久化
		false,          // 不自动删除
		false,          // 不排他
		false,          // 不等待
		nil,
	)
	if err != nil {
		fmt.Printf("⚠️  队列声明失败: %v\n", err)
		conn.Close()
		return
	}

	// 队列绑定到交换机，只接收 routingKey = "mail.welcome" 的消息
	err = ch.QueueBind(
		"mail.welcome",      // 队列
		"mail.welcome",      // routingKey
		"splatoon.events",   // 交换机
		false,
		nil,
	)
	if err != nil {
		fmt.Printf("⚠️  队列绑定失败: %v\n", err)
		conn.Close()
		return
	}

	RabbitMQConn = conn
	RabbitMQCh = ch
	fmt.Println("✅ RabbitMQ 连接成功")
}

// PublishEvent 向交换机发布消息（给其他服务/worker 异步消费）
func PublishEvent(routingKey string, payload interface{}) {
	if RabbitMQCh == nil {
		return
	}
	data, _ := json.Marshal(payload)
	err := RabbitMQCh.Publish(
		"splatoon.events", // 交换机
		routingKey,        // routingKey
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // 持久化消息
			Body:         data,
		},
	)
	if err != nil {
		fmt.Printf("⚠️  事件发布失败: %v\n", err)
	}
}

// CloseRabbitMQ 关闭连接（main 退出时调用）
func CloseRabbitMQ() {
	if RabbitMQCh != nil {
		RabbitMQCh.Close()
	}
	if RabbitMQConn != nil {
		RabbitMQConn.Close()
	}
}
