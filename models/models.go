package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email    string `gorm:"unique;not null"`
	Password string `gorm:"not null" `
	UserName string `gorm:"not null" `
}

type Party struct {
	gorm.Model
	Title     string
	Game      string
	Playernum int
	MaxNum    int
	OwnerID   string
	OwnerName string
}

type PartyMember struct {
	gorm.Model
	PartyID int
	UserID  int
	status  string
}
