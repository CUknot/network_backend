package models

import (
	"time"
)

type Room struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	CreatedBy uint      `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Users     []User    `gorm:"many2many:room_users;" json:"users,omitempty"`
	Messages  []Message `json:"messages,omitempty"`
}

type RoomUser struct {
	RoomID    uint      `gorm:"primaryKey" json:"room_id"`
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
