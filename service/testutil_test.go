package service

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"splatoon-backend/config"
	"splatoon-backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB 每个测试新建一个隔离的共享内存库（纯 Go sqlite，无需 CGO），并赋给 config.Db。
// · 用唯一的命名内存库（file:...?mode=memory&cache=shared）：测试间互不污染，
//
//	否则 file::memory:?cache=shared 会复用整个进程共享的同一块内存库，导致数据串测。
//
// · shared-cache 让单个测试内的多个连接共享同一份数据，才能支撑并发行程。
func newTestDB(t testing.TB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 测试环境关掉 SQL 日志，降噪且贴近真实 IO
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Party{}, &models.PartyMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(8)
		sqlDB.SetMaxIdleConns(8)
	}
	config.Db = db
}

// reloadParty 重新读取 party，用于断言 playernum 等字段的最新值。
func reloadParty(t testing.TB, id uint) models.Party {
	t.Helper()
	var p models.Party
	if err := config.Db.First(&p, id).Error; err != nil {
		t.Fatalf("reload party %d: %v", id, err)
	}
	return p
}

func uintStr(id uint) string { return strconv.FormatUint(uint64(id), 10) }
func itoa(i int) string      { return strconv.Itoa(i) }
func nowUTC() time.Time      { return time.Now().UTC() }
