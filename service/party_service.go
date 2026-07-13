package service

import (
	"errors"
	"splatoon-backend/config"
	"splatoon-backend/models"
	"splatoon-backend/utils"
	"strconv"

	"gorm.io/gorm"
)

type CreatePartyReq struct {
	Title        string `json:"title"`
	Game         string `json:"game"`
	Introduction string `json:"introduction"`
	Playernum    int    `json:"playernum"`
	MaxNum       int    `json:"maxNum"`
}

type MemberInfo struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	Gender   string `json:"gender"`
	Status   string `json:"status"`
}

type PartyDetail struct {
	models.Party
	Members []MemberInfo `json:"members"`
}

func CreateParty(userID string, req *CreatePartyReq) (*models.Party, error) {
	// 查 DB 拿用户名
	var user models.User
	if err := config.Db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}

	party := &models.Party{
		Title:        req.Title,
		Game:         req.Game,
		Introduction: req.Introduction,
		Playernum:    1,
		MaxNum:       req.MaxNum,
		OwnerID:      userID,
		OwnerName:    user.UserName,
	}

	if err := config.Db.Create(party).Error; err != nil {
		return nil, err
	}

	// 创建者自动加入成员表
	member := &models.PartyMember{
		PartyID: party.ID,
		UserID:  userID,
		Status:  "JOINED",
	}
	if err := config.Db.Create(member).Error; err != nil {
		return nil, err
	}

	return party, nil
}

func GetPartyList() ([]models.Party, error) {
	var parties []models.Party
	if err := config.Db.Order("created_at desc").Find(&parties).Error; err != nil {
		return nil, err
	}
	return parties, nil
}

func GetPartyByID(partyID string) (*models.Party, error) {
	var party models.Party
	if err := config.Db.Where("id = ?", partyID).First(&party).Error; err != nil {
		return nil, errors.New("party not found")
	}
	return &party, nil
}

func GetPartyDetail(partyID string) (*PartyDetail, error) {
	party, err := GetPartyByID(partyID)
	if err != nil {
		return nil, err
	}

	var partyMembers []models.PartyMember
	config.Db.Where("party_id = ?", party.ID).Find(&partyMembers)

	members := []MemberInfo{}
	for _, pm := range partyMembers {
		var user models.User
		if err := config.Db.Where("id = ?", pm.UserID).First(&user).Error; err != nil {
			continue
		}
		members = append(members, MemberInfo{
			UserID:   pm.UserID,
			UserName: user.UserName,
			Gender:   user.Gender,
			Status:   pm.Status,
		})
	}

	return &PartyDetail{
		Party:   *party,
		Members: members,
	}, nil
}

func DeleteParty(partyid string, userid string) (*models.Party, error) {
	var exist models.Party
	if err := config.Db.Where("id=?", partyid).First(&exist).Error; err != nil {
		return nil, err
	}
	owner := exist.OwnerID
	if owner == userid {
		// 软删该 party 下所有成员记录
		if err := config.Db.Where("party_id = ?", exist.ID).Delete(&models.PartyMember{}).Error; err != nil {
			return nil, err
		}
		if err := config.Db.Where("id=?", partyid).Delete(&exist).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("only owner have right")
	}
	return &exist, nil
}

func JoinParty(userID string, partyID string) (*models.PartyMember, error) {
	if utils.RuserinP(userID, partyID) {
		return nil, errors.New("You are already in the party")
	}
	//格式转换string->uint
	id, err := strconv.ParseUint(partyID, 10, 64)
	if err != nil {
		return nil, errors.New("invalid party id")
	}
	member := models.PartyMember{
		PartyID: uint(id),
		UserID:  userID,
		Status:  "JOINED",
	}
	if err := config.Db.Create(&member).Error; err != nil {
		return nil, errors.New("Create partymember failed")
	}

	// 加入后当前人数 +1
	config.Db.Model(&models.Party{}).Where("id = ?", id).Update("playernum", gorm.Expr("playernum + 1"))
	return &member, nil
}

func LeaveParty(userID string, partyID string) (*models.PartyMember, error) {
	if !utils.RuserinP(userID, partyID) {
		return nil, errors.New("You are not in the party")
	}
	id, err := strconv.ParseUint(partyID, 10, 64)
	if err != nil {
		return nil, errors.New("invalid party id")
	}
	// 硬删（退队记录留着没意义，避免唯一索引冲突无法再加）
	if err := config.Db.Unscoped().Where("party_id = ? AND user_id = ?", id, userID).Delete(&models.PartyMember{}).Error; err != nil {
		return nil, errors.New("Leave party failed")
	}
	config.Db.Model(&models.Party{}).Where("id = ?", id).Update("playernum", gorm.Expr("playernum - 1"))
	return nil, nil
}
