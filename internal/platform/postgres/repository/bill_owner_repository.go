package repository

import "time"

type billOwnerModel struct {
	ID                 string     `gorm:"type:varchar(255);primaryKey"`
	Celphone           *string    `gorm:"type:varchar(50)"`
	Email              string     `gorm:"type:varchar(255);not null"`
	Name               string     `gorm:"type:varchar(255);not null"`
	IdentificationType *string    `gorm:"type:varchar(50);column:identification_type"`
	CreatedAt          time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt          *time.Time `gorm:"type:timestamp"`
}

func (billOwnerModel) TableName() string {
	return "bill_owners"
}
