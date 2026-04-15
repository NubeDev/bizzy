// Package auth provides bearer token authentication middleware for gin.
package auth

import (
	"log"
	"net/http"
	"strings"

	"github.com/NubeDev/bizzy/pkg/jsondb"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	userKey      contextKey = "auth_user"
	actingAsKey  contextKey = "acting_as"
)

// Middleware returns a gin middleware that resolves bearer tokens to users.
// In dev mode (no Authorization header), it falls back to the first user in the DB.
func Middleware(users *jsondb.Collection[models.User]) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")

		var user models.User
		var ok bool

		if strings.HasPrefix(header, "Bearer ") {
			token := strings.TrimPrefix(header, "Bearer ")
			user, ok = users.FindOne(func(u models.User) bool {
				return u.Token == token
			})
			if !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
				return
			}
		} else {
			// Dev mode: no token provided, use the first user.
			all := users.All()
			if len(all) == 0 {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "no users exist — POST /bootstrap first"})
				return
			}
			user = all[0]
		}

		// Store the authenticated user.
		c.Set(string(userKey), user)

		// Admin impersonation via X-Act-As-User header.
		if actAsID := c.GetHeader("X-Act-As-User"); actAsID != "" {
			if user.Role != models.RoleAdmin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "only admins can use X-Act-As-User"})
				return
			}
			target, ok := users.Get(actAsID)
			if !ok {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "target user not found"})
				return
			}
			log.Printf("[auth] admin %s (%s) acting as user %s (%s)", user.ID, user.Name, target.ID, target.Name)
			c.Set(string(actingAsKey), target)
		}

		c.Next()
	}
}

// GetUser returns the effective user for this request (impersonated user if applicable, otherwise authenticated user).
func GetUser(c *gin.Context) models.User {
	if v, ok := c.Get(string(actingAsKey)); ok {
		return v.(models.User)
	}
	return c.MustGet(string(userKey)).(models.User)
}

// GetAuthenticatedUser returns the actual authenticated user (ignoring impersonation).
func GetAuthenticatedUser(c *gin.Context) models.User {
	return c.MustGet(string(userKey)).(models.User)
}

// RequireAdmin is a middleware that rejects non-admin users.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetAuthenticatedUser(c)
		if user.Role != models.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}
