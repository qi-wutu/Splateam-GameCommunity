package middlewares

import (
	"net/http"
	"splatoon-backend/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader("Authorization")
		if token == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			ctx.Abort()
			return
		}
		userID, err := utils.ParseJWT(token)
		if err != nil {
			ctx.JSON(400, gin.H{"error": "ParseJWT failed"})
			ctx.Abort()
			return
		}
		ctx.Set("userID", userID)
		ctx.Next()
	}
}
