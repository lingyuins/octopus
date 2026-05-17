package model

const DefaultChannelGroupName = "Default"

type ChannelGroup struct {
	ID        int    `json:"id" gorm:"primaryKey"`
	Name      string `json:"name" gorm:"unique;not null"`
	IsDefault bool   `json:"is_default" gorm:"not null;default:false"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime:milli"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime:milli"`
}
