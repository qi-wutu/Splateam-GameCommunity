package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email    string `gorm:"unique;not null"`
	Password string `gorm:"not null" `
	UserName string `gorm:"not null" `
	Gender   string `gorm:"type:varchar(10);default:'unspecified'"`
}

type Party struct {
	gorm.Model
	Title        string `gorm:"not null"`
	Game         string `gorm:"not null"`
	Introduction string `gorm:"type:text"`
	Playernum    int    `gorm:"not null"`
	MaxNum       int    `gorm:"not null"`
	OwnerID      string `gorm:"type:varchar(191);not null;index"`
	OwnerName    string `gorm:"not null"`
}

type PartyMember struct {
	gorm.Model
	PartyID uint   `gorm:"not null;uniqueIndex:idx_party_user"`
	UserID  string `gorm:"type:varchar(191);not null;uniqueIndex:idx_party_user"`
	Status  string `gorm:"not null;default:JOINED"`
}
