package model

type BusinessCardLike struct {
	Base
	CardID int64 `gorm:"column:card_id" json:"cardId"`
	UserID int64 `gorm:"column:user_id" json:"userId"`
}

func (BusinessCardLike) TableName() string { return "business_card_like" }
