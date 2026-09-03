package service

import (
	"testing"

	"splatoon-backend/config"
	"splatoon-backend/models"
	"splatoon-backend/utils"

	"golang.org/x/crypto/bcrypt"
)

// mustCreateUser 直接落库一个用户，密码经 bcrypt 哈希（cost 低一些以加快测试）。
func mustCreateUser(t *testing.T, email, username, password string, active bool) models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{
		Email:    email,
		Password: string(hash),
		UserName: username,
		Gender:   "unspecified",
		Active:   active,
	}
	if err := config.Db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestRegisterUser_CreatesInactiveHashedUser(t *testing.T) {
	newTestDB(t)

	res, err := Register_user("a@x.com", "userA", "secret123", "male")
	if err != nil {
		t.Fatalf("Register_user: %v", err)
	}
	if res.User.Username != "userA" || res.User.Email != "a@x.com" {
		t.Fatalf("unexpected result: %+v", res)
	}

	var u models.User
	if err := config.Db.Where("email = ?", "a@x.com").First(&u).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if u.Active {
		t.Fatal("newly registered user should not be active")
	}
	if u.Password == "secret123" {
		t.Fatal("password must be hashed, not stored in plaintext")
	}
	if !utils.CheckPassword("secret123", u.Password) {
		t.Fatal("bcrypt hash should verify the original password")
	}
}

func TestRegisterUser_DuplicateEmail(t *testing.T) {
	newTestDB(t)
	if _, err := Register_user("dup@x.com", "u1", "secret123", ""); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := Register_user("dup@x.com", "u2", "another123", ""); err == nil {
		t.Fatal("expected duplicate email error")
	}
}

func TestLoginUser_InactiveRejected(t *testing.T) {
	newTestDB(t)
	mustCreateUser(t, "in@x.com", "u", "secret123", false)

	if _, err := Login_user("in@x.com", "secret123"); err == nil {
		t.Fatal("expected inactive login to fail")
	}
}

func TestLoginUser_WrongPasswordRejected(t *testing.T) {
	newTestDB(t)
	mustCreateUser(t, "p@x.com", "u", "secret123", true)

	if _, err := Login_user("p@x.com", "wrong"); err == nil {
		t.Fatal("expected wrong password error")
	}
}

func TestLoginUser_SuccessReturnsJWT(t *testing.T) {
	newTestDB(t)
	u := mustCreateUser(t, "ok@x.com", "u", "secret123", true)

	res, err := Login_user("ok@x.com", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Token == "" {
		t.Fatal("expected non-empty token")
	}
	uid, err := utils.ParseJWT(res.Token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if uid != uintStr(u.ID) {
		t.Fatalf("token user_id = %s, want %s", uid, uintStr(u.ID))
	}
}

func TestActivateUser(t *testing.T) {
	newTestDB(t)
	mustCreateUser(t, "act@x.com", "u", "secret123", false)

	code, err := GenerateActivationCode("act@x.com")
	if err != nil {
		t.Fatalf("gen code: %v", err)
	}
	if err := ActivateUser("act@x.com", code); err != nil {
		t.Fatalf("activate: %v", err)
	}
	var u models.User
	config.Db.Where("email = ?", "act@x.com").First(&u)
	if !u.Active {
		t.Fatal("user should be active after activation")
	}
	// 激活码一次性，重复使用应失败
	if err := ActivateUser("act@x.com", code); err == nil {
		t.Fatal("expected reusing a consumed code to fail")
	}
	// 错误激活码应失败
	if err := ActivateUser("act@x.com", "000000"); err == nil {
		t.Fatal("expected wrong code to fail")
	}
}

func TestGetUserByID(t *testing.T) {
	newTestDB(t)
	u := mustCreateUser(t, "g@x.com", "u", "secret123", true)

	got, err := GetUserByID(uintStr(u.ID))
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.ID != u.ID || got.Email != "g@x.com" {
		t.Fatalf("unexpected profile: %+v", got)
	}
	if _, err := GetUserByID("999999"); err == nil {
		t.Fatal("expected not-found error")
	}
}
