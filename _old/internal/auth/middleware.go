package auth

import (
	"net/http"
	"strings"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"

	"github.com/gin-gonic/gin"
)

// AuthWithUserMiddleware creates a middleware for JWT authentication, fetching user from DB, handling redirects, and setting user object in Gin context
func (s *Service) AuthWithUserMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Log.Debug().Str("path", c.Request.URL.Path).Msg("AuthWithUserMiddleware invoked")
		// Log cookies and headers for debugging
		logger.Log.Debug().Interface("cookies", c.Request.Cookies()).Msg("Request cookies")
		logger.Log.Debug().Str("header", c.GetHeader("Authorization")).Msg("Authorization header")

		authHeader := c.GetHeader("Authorization")
		var token string
		if authHeader != "" {
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
				token = tokenParts[1]
			}
		}
		if token == "" {
			cookieToken, err := c.Cookie("auth_token")
			logger.Log.Debug().Str("value", cookieToken).Err(err).Msg("auth_token cookie")
			if err == nil && cookieToken != "" {
				token = cookieToken
			}
		}
		logger.Log.Debug().Str("token", token).Msg("JWT token used for validation")
		if token == "" {
			logger.Log.Debug().Msg("No JWT token found, redirecting to login")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		claims, err := s.ValidateJWT(token)
		logger.Log.Debug().Interface("claims", claims).Err(err).Msg("ValidateJWT result")
		if err != nil || claims.UserID == "" {
			logger.Log.Debug().Msg("JWT validation failed or user_id is empty redirecting to login")
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)

		user, err := s.UserRepo.FindByIdentifier(claims.UserID)
		logger.Log.Debug().Interface("user", user).Err(err).Msg("FindByIdentifier result")
		if err != nil || user == nil {
			logger.Log.Error().Str("user_id", claims.UserID).Err(err).Msg("AuthWithUserMiddleware: user not found in DB, creating new user")
			user = &models.User{
				PlexID:  claims.UserID,
				Email:   claims.Email,
				Name:    claims.Name,
				Picture: "",
			}
			createdUser, createErr := s.UserRepo.CreatePlexUser(user)
			if createErr != nil {
				logger.Log.Error().Str("user_id", claims.UserID).Err(createErr).Msg("Failed to create Plex user on first login")
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
			user = createdUser
		}
		c.Set("user_obj", user)
		c.Next()
	}
}

// GetUserFromContext retrieves user information from the Gin context
func GetUserFromContext(c *gin.Context) (userID string, email string, name string, exists bool) {
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		logger.Log.Debug().Str("path", c.Request.URL.Path).Str("method", c.Request.Method).Msg("GetUserFromContext: user_id not found in context")
		return "", "", "", false
	}

	emailInterface, _ := c.Get("user_email")
	nameInterface, _ := c.Get("user_name")
	userID, ok := userIDInterface.(string)
	if !ok {
		logger.Log.Debug().Msgf("GetUserFromContext: user_id type assertion failed: %v", userIDInterface)
		return "", "", "", false
	}

	email, _ = emailInterface.(string)
	name, _ = nameInterface.(string)
	logger.Log.Debug().Msgf("GetUserFromContext: user_id=%v, email=%v, name=%v, exists=%v", userID, email, name, exists)
	return userID, email, name, true
}
