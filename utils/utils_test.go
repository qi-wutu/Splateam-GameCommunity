package utils

import (
	"strings"
	"testing"

	"splatoon-backend/config"

	"golang.org/x/crypto/bcrypt"
)

func TestJWTRoundtrip(t *testing.T) {
	config.JwtSecret = strings.Repeat("k", 32)

	token, err := GenerateJWT("42")
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	// 裸 token
	if uid, err := ParseJWT(token); err != nil || uid != "42" {
		t.Fatalf("parse bare token: uid=%q err=%v", uid, err)
	}
	// 带 "Bearer " 前缀
	if uid, err := ParseJWT("Bearer " + token); err != nil || uid != "42" {
		t.Fatalf("parse bearer token: uid=%q err=%v", uid, err)
	}
	// 其它签名算法应被拒绝（伪造 none 算法等）
	if _, err := ParseJWT("garbage.token.value"); err == nil {
		t.Fatal("expected malformed token to fail")
	}
}

func TestJWTWrongSecretRejected(t *testing.T) {
	config.JwtSecret = strings.Repeat("k", 32)
	token, err := GenerateJWT("42")
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	// 换一个密钥，签名校验必须失败
	config.JwtSecret = strings.Repeat("z", 32)
	if _, err := ParseJWT(token); err == nil {
		t.Fatal("expected signature mismatch to fail")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), 4)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !CheckPassword("secret123", string(hash)) {
		t.Fatal("expected correct password to verify")
	}
	if CheckPassword("wrong", string(hash)) {
		t.Fatal("expected wrong password to fail")
	}
}
