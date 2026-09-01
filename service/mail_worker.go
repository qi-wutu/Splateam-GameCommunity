package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"sync"
	"time"

	"splatoon-backend/config"
)

// ---------- 邮件任务结构 ----------

type MailTask struct {
	Type     string `json:"type"`     // activation_code
	Email    string `json:"email"`
	UserName string `json:"userName"`
	Code     string `json:"code,omitempty"`
}

// ---------- 激活码 ----------

const (
	activationTTL  = 30 * time.Minute
	redisActPrefix = "activation:"
)

// 内存 fallback（Redis 不可用时使用）
var (
	memCodesMux sync.Mutex
	memCodes    = map[string]memCode{}
)

type memCode struct {
	code      string
	expiresAt time.Time
}

// GenerateActivationCode 生成随机 6 位激活码，优先存 Redis，失败时存内存
func GenerateActivationCode(email string) (string, error) {
	bytes := make([]byte, 3)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	code := hex.EncodeToString(bytes)

	if config.RedisClient != nil {
		config.RedisClient.Set(config.RedisCtx, redisActPrefix+email, code, activationTTL)
		return code, nil
	}

	// Redis 不可用 → 存内存
	memCodesMux.Lock()
	memCodes[email] = memCode{code: code, expiresAt: time.Now().Add(activationTTL)}
	memCodesMux.Unlock()
	return code, nil
}

// VerifyActivationCode 校验激活码
func VerifyActivationCode(email, code string) bool {
	if config.RedisClient != nil {
		stored, err := config.RedisClient.Get(config.RedisCtx, redisActPrefix+email).Result()
		if err != nil || stored != code {
			return false
		}
		config.RedisClient.Del(config.RedisCtx, redisActPrefix+email)
		return true
	}

	// Redis 不可用 → 查内存
	memCodesMux.Lock()
	defer memCodesMux.Unlock()
	entry, ok := memCodes[email]
	if !ok || entry.code != code || time.Now().After(entry.expiresAt) {
		delete(memCodes, email) // 清理过期
		return false
	}
	delete(memCodes, email)
	return true
}

// ---------- MQ Consumer ----------

// StartMailWorker 启动邮件消费者（在 goroutine 中运行）
func StartMailWorker() {
	if config.RabbitMQCh == nil {
		fmt.Println("⚠️  RabbitMQ 未连接，邮件 worker 不启动")
		fmt.Println("  激活码将直接输出到控制台")
		return
	}

	msgs, err := config.RabbitMQCh.Consume(
		"mail.welcome",
		"",
		false, // auto-ack false → 手动确认
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		fmt.Printf("⚠️  邮件队列消费失败: %v\n", err)
		return
	}

	fmt.Println("📧  邮件 worker 已启动，等待发信任务...")

	for msg := range msgs {
		var task MailTask
		if err := json.Unmarshal(msg.Body, &task); err != nil {
			msg.Nack(false, false)
			continue
		}
		switch task.Type {
		case "activation_code":
			sendActivationCodeEmail(task.Email, task.UserName, task.Code)
		default:
			log.Printf("未知邮件类型: %s", task.Type)
		}
		msg.Ack(false)
	}
}

// ---------- SMTP 发邮件 ----------

var smtpHost, smtpPort, smtpUser, smtpPass string

func initSMTP() {
	smtpHost = config.GetEnv("SMTP_HOST", "smtp.qq.com")
	smtpPort = config.GetEnv("SMTP_PORT", "587")
	smtpUser = config.GetEnv("SMTP_USER", "")
	smtpPass = config.GetEnv("SMTP_PASSWORD", "")
}

func sendMail(to, subject, body string) error {
	initSMTP()
	if smtpUser == "" {
		return fmt.Errorf("SMTP 未配置")
	}

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	encodedSubj := base64.StdEncoding.EncodeToString([]byte(subject))
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: =?UTF-8?B?%s?=\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		smtpUser, to, encodedSubj)
	msg := []byte(header + body)

	return smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{to}, msg)
}

func sendActivationCodeEmail(to, userName, code string) {
	subject := "您的 Splatoon 激活码"
	body := fmt.Sprintf(`您好 %s！

您的激活码是：%s

请在注册页面输入此激活码完成注册。
激活码有效期为 30 分钟，过期需重新注册。

如果非您本人操作，请忽略此邮件。

Splatoon 组队平台团队`, userName, code)

	if err := sendMail(to, subject, body); err != nil {
		log.Printf("❌ 激活码邮件发送失败 [%s]: %v", to, err)
	} else {
		log.Printf("✅ 激活码邮件已发送 [%s]", to)
	}
}
