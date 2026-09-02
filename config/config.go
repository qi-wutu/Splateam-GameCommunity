package config

import (
	"errors"
	"fmt"
	"os"
	"splatoon-backend/models"
	"time"

	"github.com/joho/godotenv"
	msql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Db *gorm.DB
var ServerPort string
var JwtSecret string

func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func LoadEnv() {
	if err := godotenv.Load("config/.env"); err != nil {
		fmt.Println("未找到 config/.env 文件，使用默认配置")
	}
	// 基础配置在 LoadEnv 阶段就绪，任何组件都直接从 config 读取，
	// 不依赖 InitMySQL 的执行顺序（此前 JWT_SECRET 只在 InitMySQL 里赋值）。
	ServerPort = GetEnv("SERVER_PORT", "8080")
	JwtSecret = GetEnv("JWT_SECRET", "splatoon-dev-secret-key")
}

// ValidateJWT 校验 JWT 签名密钥强度。应在 LoadEnv 之后调用。
// 空密钥会直接导致认证被任意伪造；仍用公开默认值或过短的密钥同样是安全隐患。
func ValidateJWT() error {
	switch {
	case JwtSecret == "":
		return errors.New("JWT_SECRET 为空：签名密钥不能为空，否则认证可被任意伪造")
	case JwtSecret == "splatoon-dev-secret-key":
		return errors.New("JWT_SECRET 仍为公开默认值 splatoon-dev-secret-key：请设置强随机密钥")
	case len(JwtSecret) < 32:
		return errors.New("JWT_SECRET 过短(<32 字符)：请使用更长的强随机密钥")
	}
	return nil
}

func InitMySQL() {
	dbHost := GetEnv("DB_HOST", "127.0.0.1")
	dbPort := GetEnv("DB_PORT", "3306")
	dbUser := GetEnv("DB_USER", "root")
	dbPass := GetEnv("DB_PASSWORD", "")
	dbName := GetEnv("DB_NAME", "splatoon")

	// 用 go-sql-driver/mysql Config 生成 DSN（修复 Windows MySQL 默认 gbk 导致中文乱码）
	cfg := msql.Config{
		User:      dbUser,
		Passwd:    dbPass,
		Net:       "tcp",
		Addr:      fmt.Sprintf("%s:%s", dbHost, dbPort),
		DBName:    dbName,
		Collation: "utf8mb4_unicode_ci",
		Loc:       time.Local,
		ParseTime: true,
		Params:    map[string]string{"charset": "utf8mb4"},
	}

	var err error
	Db, err = gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		panic("数据库连接失败：" + err.Error())
	}

	sqldb, err := Db.DB()
	sqldb.SetMaxOpenConns(100)
	sqldb.SetMaxIdleConns(10)
	sqldb.SetConnMaxLifetime(time.Hour)

	if err != nil {
		panic("Error setting database connection pool:" + err.Error())
	}

	fmt.Println("MySQL 连接成功")
	Db.AutoMigrate(&models.User{}, &models.Party{}, &models.PartyMember{})
}
