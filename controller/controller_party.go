package controller

import (
	"net/http"
	"splatoon-backend/service"

	"github.com/gin-gonic/gin"
)

func PartyList(ctx *gin.Context) {
	parties, err := service.GetPartyList()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, parties)
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
