package service

import (
	"errors"
	"fmt"
	"splatoon-backend/config"
	"splatoon-backend/models"
	"splatoon-backend/utils"

	"golang.org/x/crypto/bcrypt"
)

type UserInfo struct {
	Username string
	Email    string
}

type Result struct {
	Token string   `json:"token"`
	User  UserInfo `json:"user"`
}

func Register_user(email string, username string, password string, gender string) (*Result, error) {
	//密码哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return nil, errors.New("hash failed")
	}
	//查询是否已经注册
	var existing models.User
	err = config.Db.Where("email = ? ", email).First(&existing).Error
	if err == nil {
		return nil, errors.New("this email has been used")
	}
	//性别默认值
	genderVal := gender
	if genderVal != "male" && genderVal != "female" {
		genderVal = "unspecified"
	}
	//数据表创建
	user := models.User{
		Email:    email,
		Password: string(hash),
		UserName: username,
		Gender:   genderVal,
	}
	fmt.Printf("DEBUG username before save: %q\n", user.UserName)
	if err := config.Db.Create(&user).Error; err != nil {
		return nil, errors.New("Create Data failed")
	}
	// 生成 JWT
	token, err := utils.GenerateJWT(fmt.Sprintf("%d", user.ID))
	if err != nil {
		return nil, errors.New("Create token failed")
	}
	return &Result{
		Token: token,
		User: UserInfo{
			Username: user.UserName,
			Email:    user.Email,
		},
	}, nil
}

type UserProfile struct {
	ID       uint   `json:"id"`
	UserName string `json:"userName"`
	Email    string `json:"email"`
	Gender   string `json:"gender"`
}

func GetUserByID(userID string) (*UserProfile, error) {
	var user models.User
	if err := config.Db.First(&user, userID).Error; err != nil {
		return nil, errors.New("user not found")
	}
	return &UserProfile{
		ID:       user.ID,
		UserName: user.UserName,
		Email:    user.Email,
		Gender:   user.Gender,
	}, nil
}

func Login_user(email string, password string) (*Result, error) {
	var exist models.User
	// 根据 email 查询用户
	err := config.Db.Where("email = ?", email).First(&exist).Error
	if err != nil {
		return nil, errors.New("Couldn't find data base on email")
	}
	// 用 bcrypt 比对密码
	if !utils.CheckPassword(password, exist.Password) {
		return nil, errors.New("Wrong password")
	}
	// 生成 JWT
	token, err := utils.GenerateJWT(fmt.Sprintf("%d", exist.ID))
	if err != nil {
		return nil, errors.New("Create token failed")
	}
	return &Result{
		Token: token,
		User: UserInfo{
			Username: exist.UserName,
			Email:    exist.Email,
		},
	}, nil
}
