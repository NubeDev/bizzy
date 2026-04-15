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
	user, ok := a.Users.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (a *API) deleteUser(c *gin.Context) {
	id := c.Param("id")
	// Prevent self-deletion.
	caller := auth.GetAuthenticatedUser(c)
	if caller.ID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot delete yourself"})
		return
	}
	if err := a.Users.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (a *API) rotateToken(c *gin.Context) {
	id := c.Param("id")
	caller := auth.GetAuthenticatedUser(c)

	// Users can rotate their own token; admins can rotate anyone's.
	if caller.Role != models.RoleAdmin && caller.ID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "can only rotate your own token (or be admin)"})
		return
	}

	user, ok := a.Users.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	user.Token = models.GenerateToken()
	if err := a.Users.Update(user); err != nil {
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

	user, ok := a.Users.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	user.Token = "" // revoked — user can no longer authenticate
	if err := a.Users.Update(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "revoked": true})
}
