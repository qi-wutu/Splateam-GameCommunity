package controller

import (
	"net/http"
	"splatoon-backend/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

// PartyList 组队列表（分页）。query: cursor=..., limit=...
// cursor 来自上一页响应的 nextCursor，第一页可省略。
func PartyList(ctx *gin.Context) {
	cursor := ctx.Query("cursor")
	limit := 0
	if l := ctx.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	page, err := service.GetPartyList(cursor, limit)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, page)
}

func GetParty(ctx *gin.Context) {
	pid := ctx.Param("id")
	detail, err := service.GetPartyDetail(pid)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, detail)
}

func CreateParty(ctx *gin.Context) {
	var req service.CreatePartyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "JSON invalid"})
		return
	}
	userIDStr, _ := ctx.Get("userID")
	party, err := service.CreateParty(userIDStr.(string), &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, party)
}

func DeleteParty(ctx *gin.Context) {
	userIDStr, _ := ctx.Get("userID")
	pid := ctx.Param("id")
	DeleteP, err := service.DeleteParty(pid, userIDStr.(string))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, DeleteP)
}

func JoinParty(ctx *gin.Context) {
	userIDStr, _ := ctx.Get("userID")
	pid := ctx.Param("id")
	JoinP, err := service.JoinParty(userIDStr.(string), pid)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, JoinP)
}

func LeaveParty(ctx *gin.Context) {
	userIDStr, _ := ctx.Get("userID")
	pid := ctx.Param("id")
	LeaveP, err := service.LeaveParty(userIDStr.(string), pid)
	if err != nil {
		ctx.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(200, LeaveP)
}
