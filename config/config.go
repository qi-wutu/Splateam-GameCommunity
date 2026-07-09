package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var Db *gorm.DB
var ServerPort string

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

	// 连接 MySQL,这里dsn直接从env中读取后拼接
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbPort, dbName)
	var err error
	Db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("数据库连接失败：" + err.Error())
	}
	fmt.Println("MySQL 连接成功")

	// 自动建表（后续加 models）
	// db.AutoMigrate(&models.User{}, &models.Party{})

}
