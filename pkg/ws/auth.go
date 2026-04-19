package ws

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthFromQuery authenticates a WebSocket connection using a ?token= query param.
// Browsers can't set headers on WS upgrade, so we use a query param.
// In dev mode (no token), falls back to the first user in the database.
//
// Returns the authenticated user and true, or writes an HTTP error and returns false.
func AuthFromQuery(c *gin.Context, db *gorm.DB) (models.User, bool) {
	var user models.User
	token := c.Query("token")

	if token != "" && token != "dev" {
		if err := db.Where("token = ?", token).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return user, false
		}
		return user, true
	}

	// Dev mode: use the first user.
	if err := db.First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no users — POST /bootstrap first"})
		return user, false
	}
	return user, true
}
