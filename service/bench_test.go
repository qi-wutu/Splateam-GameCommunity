package service

import (
	"strings"
	"testing"

	"splatoon-backend/config"
	"splatoon-backend/utils"
)

// 以下微基准在"本机内存 sqlite"上跑，用于横向比较各热路径的耗时量级，
// 不代表生产 MySQL 的绝对吞吐（真实压测走 ws_test 的 k6）。
// 运行：go test ./service/ -bench . -benchmem -run '^$'

// BenchmarkJoinParty 单线程连续加入：一次完整 join = 事务内原子 UPDATE + INSERT 成员。
func BenchmarkJoinParty(b *testing.B) {
	newTestDB(b)
	p := mustCreateParty(b, 1<<30, 0) // 容量巨大，保证全部成功
	i := 0
	b.ReportAllocs()
	for b.Loop() {
		if _, err := JoinParty(itoa(i), uintStr(p.ID)); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkCursorEncode(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		encodePartyCursor(nowUTC(), uint(1))
	}
}

func BenchmarkCursorDecode(b *testing.B) {
	c := encodePartyCursor(nowUTC(), 42)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := parsePartyCursor(c); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJWTParse(b *testing.B) {
	config.JwtSecret = strings.Repeat("k", 32)
	token, err := utils.GenerateJWT("42")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := utils.ParseJWT(token); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJWTSign(b *testing.B) {
	config.JwtSecret = strings.Repeat("k", 32)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := utils.GenerateJWT("42"); err != nil {
			b.Fatal(err)
		}
	}
}
