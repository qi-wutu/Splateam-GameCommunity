package utils

import (
	"errors"
	"splatoon-backend/config"
	"splatoon-backend/models"
	"time"

	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

func GenerateJWT(username string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(time.Hour * 72).Unix(),
	})
	signedToken, err := token.SignedString([]byte("secret"))
	return "Bearer " + signedToken, err
}
func CheckPassword(password string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// 验证JWT
func ParseJWT(tokenString string) (string, error) {
	//去除bearer前缀
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}
	//通过匿名函数解析token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("Unexpected Signing Method")
		}
		return []byte("secret"), nil
	})
	if err != nil {
		return "", err
	}
	//第一行类型强转，然后找Username
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username, ok := claims["username"].(string)
		if !ok {
			return "", errors.New("Username claim is not success")
		}
		return username, nil
	}
	return "", err

}

func RuserinP(userID string, partyID string) bool {
	var member models.PartyMember
	err := config.Db.Where("user_id = ? AND party_id = ?", userID, partyID).First(&member).Error
	return err == nil
}
