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
	//数据表创建（未激活）
	user := models.User{
		Email:    email,
		Password: string(hash),
		UserName: username,
		Gender:   genderVal,
		Active:   false,
	}
	fmt.Printf("DEBUG username before save: %q\n", user.UserName)
	if err := config.Db.Create(&user).Error; err != nil {
		return nil, errors.New("Create Data failed")
	}

	// 生成激活码 → 存 Redis → MQ 发邮件
	code, err := GenerateActivationCode(email)
	if err == nil && config.RabbitMQCh != nil {
		config.PublishEvent("mail.welcome", MailTask{
			Type:     "activation_code",
			Email:    email,
			UserName: username,
			Code:     code,
		})
	} else if err == nil && config.RabbitMQCh == nil {
		// 开发环境没有 MQ 时，激活码打印到控制台
		fmt.Printf("📌 激活码 [%s]: %s（无 MQ，直接输出到控制台）\n", email, code)
	}

	return &Result{
		Token: "",
		User: UserInfo{
			Username: user.UserName,
			Email:    user.Email,
		},
	}, nil
}

// ActivateUser 校验激活码并激活用户
func ActivateUser(email, code string) error {
	if !VerifyActivationCode(email, code) {
		return errors.New("激活码无效或已过期")
	}
	result := config.Db.Model(&models.User{}).Where("email = ? AND active = ?", email, false).
		Update("active", true)
	if result.Error != nil {
		return errors.New("激活失败")
	}
	if result.RowsAffected == 0 {
		return errors.New("用户不存在或已被激活")
	}
	return nil
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
	// 检查邮箱是否已激活
	if !exist.Active {
		return nil, errors.New("邮箱尚未激活，请先查看邮件输入激活码")
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
