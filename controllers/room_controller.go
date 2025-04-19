package controllers

import (
	"net/http"
	"strconv"

	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/models"
	"github.com/gin-gonic/gin"
)

type CreateRoomInput struct {
	Name    string `json:"name" binding:"required" example:"General Chat"`
	UserIDs []uint `json:"user_ids"`
}

type UpdateRoomInput struct {
	Name    string `json:"name" example:"Updated Chat Room"`
	UserIDs []uint `json:"user_ids"`
}

// GetRooms godoc
// @Summary Get all rooms for the authenticated user
// @Description Returns all chat rooms that the authenticated user is a member of
// @Tags rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "List of rooms"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/rooms [get]
func GetRooms(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var roomUsers []models.RoomUser
	if err := database.DB.Where("user_id = ?", userID).Find(&roomUsers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch room user data"})
		return
	}

	roomIDs := make([]uint, 0, len(roomUsers))
	lastReadMap := make(map[uint]models.RoomUser)
	for _, ru := range roomUsers {
		roomIDs = append(roomIDs, ru.RoomID)
		lastReadMap[ru.RoomID] = ru
	}

	var rooms []models.Room
	if err := database.DB.Preload("Users").Where("id IN ?", roomIDs).Find(&rooms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rooms"})
		return
	}

	// Build the response with lastReadAt and unreadCount
	response := []gin.H{}
	for _, room := range rooms {
		lastRead := lastReadMap[room.ID].LastReadAt

		var unreadCount int64
		database.DB.Model(&models.Message{}).
			Where("room_id = ? AND created_at > ?", room.ID, lastRead).
			Count(&unreadCount)

		response = append(response, gin.H{
			"room":        room,
			"lastReadAt":  lastRead,
			"unreadCount": unreadCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"rooms": response})
}

// CreateRoom godoc
// @Summary Create a new chat room
// @Description Creates a new chat room with the authenticated user as a member
// @Tags rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param room body CreateRoomInput true "Room Creation"
// @Success 201 {object} map[string]interface{} "Room created successfully"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/rooms [post]
func CreateRoom(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input CreateRoomInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create room
	room := models.Room{
		Name:      input.Name,
		CreatedBy: userID,
	}

	if err := database.DB.Create(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create room"})
		return
	}

	// Add creator to room
	roomUser := models.RoomUser{
		RoomID: room.ID,
		UserID: userID,
	}
	if err := database.DB.Create(&roomUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to room"})
		return
	}

	// Add other users to room if provided
	for _, id := range input.UserIDs {
		if id == userID {
			continue // Skip creator as they're already added
		}

		roomUser := models.RoomUser{
			RoomID: room.ID,
			UserID: id,
		}
		database.DB.Create(&roomUser)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Room created successfully",
		"room":    room,
	})
}

// GetRoom godoc
// @Summary Get details of a specific room
// @Description Returns details of a specific chat room
// @Tags rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Room ID"
// @Success 200 {object} map[string]interface{} "Room details"
// @Failure 400 {object} map[string]string "Invalid room ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Room not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/rooms/{id} [get]
func GetRoom(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	roomID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Get RoomUser for LastReadAt
	var roomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", roomID, userID).First(&roomUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this room"})
		return
	}

	var room models.Room
	if err := database.DB.Preload("Users").First(&room, roomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	// Count unread messages
	var unreadCount int64
	database.DB.Model(&models.Message{}).
		Where("room_id = ? AND created_at > ?", roomID, roomUser.LastReadAt).
		Count(&unreadCount)

	c.JSON(http.StatusOK, gin.H{
		"room":        room,
		"lastReadAt":  roomUser.LastReadAt,
		"unreadCount": unreadCount,
	})
}

// UpdateRoom godoc
// @Summary Update a room's details
// @Description Updates a room's name and/or members
// @Tags rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Room ID"
// @Param room body UpdateRoomInput true "Room Update"
// @Success 200 {object} map[string]string "Room updated successfully"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/rooms/{id} [put]
func UpdateRoom(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	roomID, err := strconv.ParseUint(c.Param("id"), 10, 32)
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

	var input UpdateRoomInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update room name if provided
	if input.Name != "" {
		if err := database.DB.Model(&models.Room{}).Where("id = ?", roomID).Update("name", input.Name).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update room"})
			return
		}
	}

	// Update room members if provided
	if input.UserIDs != nil {
		// Remove all existing members except the creator
		if err := database.DB.Where("room_id = ? AND user_id != ?", roomID, userID).Delete(&models.RoomUser{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update room members"})
			return
		}

		// Add new members
		for _, id := range input.UserIDs {
			if id == userID {
				continue // Skip creator as they're already a member
			}

			roomUser := models.RoomUser{
				RoomID: uint(roomID),
				UserID: id,
			}
			database.DB.Create(&roomUser)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Room updated successfully"})
}

// DeleteRoom godoc
// @Summary Delete a room
// @Description Deletes a room and all its messages
// @Tags rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Room ID"
// @Success 200 {object} map[string]string "Room deleted successfully"
// @Failure 400 {object} map[string]string "Invalid room ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Room not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/rooms/{id} [delete]
func DeleteRoom(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	roomID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid room ID"})
		return
	}

	// Check if user is the creator of the room
	var room models.Room
	if err := database.DB.First(&room, roomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	if room.CreatedBy != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the room creator can delete the room"})
		return
	}

	// Delete room users
	if err := database.DB.Where("room_id = ?", roomID).Delete(&models.RoomUser{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete room members"})
		return
	}

	// Delete messages
	if err := database.DB.Where("room_id = ?", roomID).Delete(&models.Message{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete room messages"})
		return
	}

	// Delete room
	if err := database.DB.Delete(&room).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Room deleted successfully"})
}

// GetUnreadCount godoc
// @Summary Get unread message count for a room
// @Description Returns the number of unread messages in a room
// @Tags rooms
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Room ID"
// @Success 200 {object} map[string]int64 "Unread message count"
// @Failure 400 {object} map[string]string "Invalid room ID"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/rooms/{id}/unread [get]
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
