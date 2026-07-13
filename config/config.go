package config

import (
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
}

func InitMySQL() {
	dbHost := GetEnv("DB_HOST", "127.0.0.1")
	dbPort := GetEnv("DB_PORT", "3306")
	dbUser := GetEnv("DB_USER", "root")
	dbPass := GetEnv("DB_PASSWORD", "")
	dbName := GetEnv("DB_NAME", "splatoon")
	ServerPort = GetEnv("SERVER_PORT", "8080")
	JwtSecret = GetEnv("JWT_SECRET", "splatoon-dev-secret-key")

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
