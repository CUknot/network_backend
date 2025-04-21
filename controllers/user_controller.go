package controllers

import (
	"net/http"
	"time"

	"github.com/CUknot/network_backend/database"
	"github.com/CUknot/network_backend/models"
	"github.com/gin-gonic/gin"
)

// SearchUsers godoc
// @Summary Search for users by username prefix
// @Description Returns users whose usernames start with the provided prefix
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username query string true "Username prefix to search for" example:"A"
// @Success 200 {object} map[string]interface{} "List of matching users"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/users [get]
func SearchUsers(c *gin.Context) {
	// Get the username prefix from query parameter
	usernamePrefix := c.Query("username")
	if usernamePrefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username prefix is required"})
		return
	}

	// Find users with usernames that start with the provided prefix
	var users []models.User
	if err := database.DB.Where("username LIKE ?", usernamePrefix+"%").
		Select("id, username, email, created_at, updated_at").
		Limit(10).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	// Return the matching users
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// GetUserProfile godoc
// @Summary Get user profile
// @Description Returns the profile of a specific user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} models.User "User profile"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/users/{id} [get]
func GetUserProfile(c *gin.Context) {
	// Get user ID from path parameter
	userID := c.Param("id")

	// Find the user
	var user models.User
	if err := database.DB.Select("id, username, email, created_at, updated_at").
		First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Return the user profile
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// GetCurrentUser godoc
// @Summary Get current user profile
// @Description Returns the profile of the authenticated user
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.User "User profile"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/users/me [get]
func GetCurrentUser(c *gin.Context) {
	// Get authenticated user ID
	userID := c.MustGet("userID").(uint)

	// Find the user
	var user models.User
	if err := database.DB.Select("id, username, email, activate_room_id, created_at, updated_at").
		First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profile"})
		return
	}

	// Return the user profile
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// SetActiveRoom godoc
// @Summary Set user's active room
// @Description Sets the room that the user is currently viewing
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param room body SetActiveRoomInput true "Active Room"
// @Success 200 {object} map[string]string "Active room set successfully"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/users/active-room [post]
func SetActiveRoom(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input SetActiveRoomInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is a member of the room
	var roomUser models.RoomUser
	if err := database.DB.Where("room_id = ? AND user_id = ?", input.RoomID, userID).
		First(&roomUser).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "You don't have access to this room"})
		return
	}

	// Update user's active room
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("activate_room_id", input.RoomID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update active room"})
		return
	}

	// Update last read time for this room
	roomUser.LastReadAt = time.Now()
	if err := database.DB.Save(&roomUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update last read time"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Active room set successfully"})
}

// ClearActiveRoom godoc
// @Summary Clear user's active room
// @Description Clears the room that the user is currently viewing
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string "Active room cleared successfully"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/users/active-room [delete]
func ClearActiveRoom(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	// Clear user's active room
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("activate_room_id", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear active room"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Active room cleared successfully"})
}
