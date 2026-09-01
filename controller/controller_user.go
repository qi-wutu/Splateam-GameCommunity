package controller

import (
	"splatoon-backend/service"

	"github.com/gin-gonic/gin"
)

func RegisterUser(ctx *gin.Context) {
	type Register_User struct {
		UserName string `json:"userName" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		Gender   string `json:"gender"`
	}
	user := Register_User{}
	//验证格式是否为json
	if err := ctx.ShouldBindJSON(&user); err != nil {
		ctx.JSON(400, gin.H{"error": "invalied json"})
		return
	}

	result, err := service.Register_user(user.Email, user.UserName, user.Password, user.Gender)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(200, result)
}

func GetCurrentUser(ctx *gin.Context) {
	userID, _ := ctx.Get("userID")
	user, err := service.GetUserByID(userID.(string))
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(200, user)
}

func GetUserOnline(ctx *gin.Context) {
	userID := ctx.Param("id")
	online := service.CheckUserOnline(userID)
	ctx.JSON(200, gin.H{"online": online})
}

func ActivateAccount(ctx *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(400, gin.H{"error": "invalid json"})
		return
	}
	if err := service.ActivateUser(req.Email, req.Code); err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"message": "激活成功，现在可以登录了"})
}

func Login(ctx *gin.Context) {
	type Login_Info struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	user := Login_Info{}
	if err := ctx.ShouldBindJSON(&user); err != nil {
		ctx.JSON(400, gin.H{"error": "invalied json"})
		return
	}
	result, err := service.Login_user(user.Email, user.Password)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(200, result)
}
