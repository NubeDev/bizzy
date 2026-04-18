package api

import (
	"net/http"

	"github.com/NubeDev/bizzy/pkg/auth"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

func (a *API) getMe(c *gin.Context) {
	user := auth.GetUser(c)
	c.JSON(http.StatusOK, user)
}

func (a *API) getUser(c *gin.Context) {
	var user models.User
	if err := a.DB.First(&user, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (a *API) deleteUser(c *gin.Context) {
	id := c.Param("id")
	caller := auth.GetAuthenticatedUser(c)
	if caller.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}
	result := a.DB.Delete(&models.User{}, "id = ?", id)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (a *API) rotateToken(c *gin.Context) {
	id := c.Param("id")
	caller := auth.GetAuthenticatedUser(c)

	if caller.Role != models.RoleAdmin && caller.ID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "can only rotate your own token (or be admin)"})
		return
	}

	var user models.User
	if err := a.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	user.Token = models.GenerateToken()
	if err := a.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "token": user.Token})
}

func (a *API) revokeToken(c *gin.Context) {
	id := c.Param("id")
	caller := auth.GetAuthenticatedUser(c)

	if caller.Role != models.RoleAdmin && caller.ID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "can only revoke your own token (or be admin)"})
		return
	}

	var user models.User
	if err := a.DB.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	user.Token = ""
	if err := a.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "revoked": true})
}
