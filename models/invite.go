package models

import (
	"time"
)

type InviteRequest struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RoomID     uint      `json:"room_id"`
	Room       Room      `gorm:"foreignKey:RoomID" json:"room,omitempty"`
	SenderID   uint      `json:"sender_id"`
	Sender     User      `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	ReceiverID uint      `json:"receiver_id"`
	Receiver   User      `gorm:"foreignKey:ReceiverID" json:"receiver,omitempty"`
	Status     string    `gorm:"size:20;default:'pending'" json:"status"` // pending, accepted, rejected
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
