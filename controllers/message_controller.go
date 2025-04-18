package controllers

import (
	"net/http"
	"strconv"

	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/models"
	"github.com/CUknot/network_backend/websocket"
	"github.com/gin-gonic/gin"
)

type CreateMessageInput struct {
	Content string `json:"content" binding:"required"`
	RoomID  uint   `json:"room_id" binding:"required"`
}

// GetMessages returns all messages for a specific room
func GetMessages(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	roomID, err := strconv.ParseUint(c.Query("room_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check if user is a member of the room
	var roomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", roomID, userID).First(&roomUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this room"})
		return
	}

	var messages []models.Message
	if err := database.DB.Where("room_id = ?", roomID).
		Order("created_at ASC").
		Preload("User").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// CreateMessage creates a new message
func CreateMessage(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input CreateMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is a member of the room
	var roomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", input.RoomID, userID).First(&roomUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this room"})
		return
	}

	// Create message
	message := models.Message{
		Content: input.Content,
		RoomID:  input.RoomID,
		UserID:  userID,
	}

	if err := database.DB.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create message"})
		return
	}

	// Load user data for the message
	database.DB.Preload("User").First(&message, message.ID)

	// Broadcast message to room
	websocket.BroadcastToRoom(input.RoomID, "message", message)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Message sent successfully",
		"data":    message,
	})
}

func GetUnreadCount(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	roomIDStr := c.Param("id")
	roomIDUint64, err := strconv.ParseUint(roomIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID format"})
		return
	}
	roomID := uint(roomIDUint64)

	// Check if user is a member of the room
	var roomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", roomID, userID).
		First(&roomUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this room"})
		return
	}

	var unreadCount int64
	if err := database.DB.Model(&models.Message{}).
		Where("room_id = ? AND created_at > ?", roomID, roomUser.LastReadAt).
		Count(&unreadCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count unread messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": unreadCount})
}
