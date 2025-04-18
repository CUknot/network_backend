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
	Content string `json:"content" binding:"required" example:"Hello, everyone!"`
	RoomID  uint   `json:"room_id" binding:"required" example:"1"`
}

// GetMessages godoc
// @Summary Get all messages for a room
// @Description Returns all messages for a specific chat room
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param room_id query int true "Room ID"
// @Success 200 {object} map[string]interface{} "List of messages"
// @Failure 400 {object} map[string]string "Invalid room ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/messages [get]
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

// CreateMessage godoc
// @Summary Create a new message
// @Description Creates a new message in a chat room
// @Tags messages
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param message body CreateMessageInput true "Message Creation"
// @Success 201 {object} map[string]interface{} "Message sent successfully"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/messages [post]
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
