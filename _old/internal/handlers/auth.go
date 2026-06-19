package handlers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"webui-skeleton/internal/auth"
	"webui-skeleton/internal/config"
	"webui-skeleton/internal/database"
	"webui-skeleton/internal/logger"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	config  *config.Config
	db      *database.DB
	authSvc *auth.Service
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(config *config.Config, db *database.DB, authSvc *auth.Service) *AuthHandler {
	return &AuthHandler{
		config:  config,
		db:      db,
		authSvc: authSvc,
	}
}

// LoginPage displays the login page
func (h *AuthHandler) LoginPage(c *gin.Context) {
	// Check if user is already authenticated
	if userID, _, _, exists := auth.GetUserFromContext(c); exists && userID != "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	data := gin.H{
		"title": "Login - WebUI Skeleton",
	}
	RenderWithHTMX(c, "login.html", data, false)
}

// PlexLogin initiates Plex PIN-based login
func (h *AuthHandler) PlexLogin(c *gin.Context) {
	logger.Log.Info().Str("client_ip", c.ClientIP()).Str("user_agent", c.Request.UserAgent()).Msg("Starting Plex login")
	forwardURL := h.config.Auth.PlexRedirectURL // Should be your app's callback URL
	authAppURL, pinID, pinCode, err := h.authSvc.AuthenticateWithPlex(c.Request.Context(), forwardURL)
	if err != nil {
		logger.Log.Error().Err(err).Str("client_ip", c.ClientIP()).Msg("Failed to start Plex login")
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}
	logger.Log.Info().Int64("pin_id", pinID).Str("pin_code", pinCode).Str("client_ip", c.ClientIP()).Msg("Plex PIN generated, redirecting to Plex Auth App")
	c.SetCookie("plex_pin_id", fmt.Sprintf("%d", pinID), 300, "/", "", false, true)
	c.SetCookie("plex_pin_code", pinCode, 300, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, authAppURL)
}

// PlexPoll polls for Plex PIN authentication and completes login
func (h *AuthHandler) PlexPoll(c *gin.Context) {
	logger.Log.Info().Str("client_ip", c.ClientIP()).Msg("Polling for Plex PIN authentication")
	pinIDStr, err := c.Cookie("plex_pin_id")
	if err != nil {
		logger.Log.Debug().Err(err).Str("client_ip", c.ClientIP()).Msg("Missing pin_id cookie (debug)")
		logger.Log.Info().Err(err).Str("client_ip", c.ClientIP()).Msg("Missing pin_id cookie (info)")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing pin_id cookie"})
		return
	}
	pinCode, err := c.Cookie("plex_pin_code")
	if err != nil {
		logger.Log.Debug().Err(err).Str("client_ip", c.ClientIP()).Msg("Missing pin_code cookie (debug)")
		logger.Log.Info().Err(err).Str("client_ip", c.ClientIP()).Msg("Missing pin_code cookie (info)")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing pin_code cookie"})
		return
	}
	var pinID int64
	_, err = fmt.Sscanf(pinIDStr, "%d", &pinID)
	if err != nil {
		logger.Log.Debug().Err(err).Str("client_ip", c.ClientIP()).Msg("Invalid pin_id (debug)")
		logger.Log.Info().Err(err).Str("client_ip", c.ClientIP()).Msg("Invalid pin_id (info)")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pin_id"})
		return
	}
	user, err := h.authSvc.CompletePlexAuthentication(c.Request.Context(), pinID, pinCode)
	if err != nil {
		logger.Log.Debug().Err(err).Int64("pin_id", pinID).Str("pin_code", pinCode).Str("client_ip", c.ClientIP()).Msg("Failed Plex authentication (debug)")
		logger.Log.Info().Err(err).Int64("pin_id", pinID).Str("pin_code", pinCode).Str("client_ip", c.ClientIP()).Msg("Failed Plex authentication (info)")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	// Save Plex token to DB
	if user.PlexToken != "" {
		err := h.authSvc.UpdatePlexToken(user.ID, user.PlexToken)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Failed to update Plex token in DB")
		}
	}
	token, err := h.authSvc.GenerateJWT(user.PlexID, user.Email, user.Name, "plex")
	if err != nil {
		logger.Log.Debug().Err(err).Int64("pin_id", pinID).Str("client_ip", c.ClientIP()).Msg("Failed to generate JWT after Plex login (debug)")
		logger.Log.Info().Err(err).Int64("pin_id", pinID).Str("client_ip", c.ClientIP()).Msg("Failed to generate JWT after Plex login (info)")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	logger.Log.Info().Int64("pin_id", pinID).Str("client_ip", c.ClientIP()).Str("user_email", user.Email).Msg("Plex login successful, JWT issued")
	c.SetCookie("auth_token", token, 3600, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

// PlexStart: returns Auth App URL for forward flow
func (h *AuthHandler) PlexStart(c *gin.Context) {
	forwardURL := h.config.Auth.PlexRedirectURL // Should be your app's callback URL, e.g. https://your-app.com/auth/plex/forward
	authAppURL, pinID, pinCode, err := h.authSvc.AuthenticateWithPlex(c.Request.Context(), forwardURL)
	if err != nil {
		logger.Log.Error().Err(err).Str("client_ip", c.ClientIP()).Msg("Failed to start Plex login")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start Plex login"})
		return
	}
	c.SetCookie("plex_pin_id", fmt.Sprintf("%d", pinID), 300, "/", "", false, true)
	c.SetCookie("plex_pin_code", pinCode, 300, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"authAppUrl": authAppURL})
}

// PlexForward: handles Plex redirect after authentication (forward flow)
func (h *AuthHandler) PlexForward(c *gin.Context) {
	logger.Log.Debug().Msg("🐛 [PlexForward] Starting Plex forward authentication flow")
	pinIDStr, err := c.Cookie("plex_pin_id")
	if err != nil {
		logger.Log.Error().Err(err).Msg("❌ [PlexForward] Missing pin_id cookie")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing pin_id cookie"})
		return
	}
	pinCode, err := c.Cookie("plex_pin_code")
	if err != nil {
		logger.Log.Error().Err(err).Msg("❌ [PlexForward] Missing pin_code cookie")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing pin_code cookie"})
		return
	}
	var pinID int64
	_, err = fmt.Sscanf(pinIDStr, "%d", &pinID)
	if err != nil {
		logger.Log.Error().Err(err).Msg("❌ [PlexForward] Invalid pin_id")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pin_id"})
		return
	}
	logger.Log.Debug().Int64("pin_id", pinID).Str("pin_code", pinCode).Msg("🔎 [PlexForward] Attempting to complete Plex authentication")
	user, err := h.authSvc.CompletePlexAuthentication(c.Request.Context(), pinID, pinCode)
	if err != nil {
		logger.Log.Error().Err(err).Int64("pin_id", pinID).Str("pin_code", pinCode).Msg("❌ [PlexForward] Failed Plex authentication")
		c.Redirect(http.StatusFound, "/")
		return
	}
	existingUser, err := h.authSvc.UserRepo.FindByPlexOrEmail(user.PlexID, user.Email)
	if err != nil {
		logger.Log.Error().Err(err).Msg("❌ [PlexForward] Failed to query user after Plex authentication")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query user"})
		return
	}
	if existingUser == nil {
		_, err := h.authSvc.UserRepo.CreatePlexUser(user)
		if err != nil {
			logger.Log.Error().Err(err).Msg("❌ [PlexForward] Failed to create user after Plex authentication")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
		logger.Log.Info().Str("plex_id", user.PlexID).Str("email", user.Email).Msg("✅ [PlexForward] New user created after Plex authentication")
	}
	token, err := h.authSvc.GenerateJWT(user.PlexID, user.Email, user.Name, "plex")
	if err != nil {
		logger.Log.Error().Err(err).Int64("pin_id", pinID).Msg("❌ [PlexForward] Failed to generate JWT after Plex login")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}
	logger.Log.Info().Int64("pin_id", pinID).Str("client_ip", c.ClientIP()).Str("user_email", user.Email).Msg("✅ [PlexForward] Plex login successful, JWT issued")
	c.SetCookie("auth_token", token, 3600, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *gin.Context) {
	logger.Log.Debug().Msg("Logout: clearing auth cookie and logging out user")
	c.SetCookie("auth_token", "", -1, "/", "", false, true)

	if c.GetHeader("Content-Type") == "application/json" {
		logger.Log.Debug().Msg("Logout: returning JSON response for logout")
		c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
	} else {
		logger.Log.Debug().Msg("Logout: redirecting to home page after logout")
		logger.Log.Info().Msg("Logout: redirecting to home page after logout")
		c.Redirect(http.StatusFound, "/")
	}
}

// GetCurrentUser returns the current authenticated user info
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, email, name, exists := auth.GetUserFromContext(c)
	if !exists {
		logger.Log.Debug().Msg("GetCurrentUser: user not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	logger.Log.Debug().Str("user_id", userID).Msg("GetCurrentUser: returning user info")
	c.JSON(http.StatusOK, gin.H{
		"id":    userID,
		"email": email,
		"name":  name,
	})
}

// GetProfile returns the full user profile
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, _, _, exists := auth.GetUserFromContext(c)
	if !exists {
		logger.Log.Debug().Msg("GetProfile: user not authenticated")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	user, err := h.authSvc.UserRepo.FindByIdentifier(userID)
	if err != nil || user == nil {
		logger.Log.Debug().Str("user_id", userID).Err(err).Msg("GetProfile: user not found")
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	logger.Log.Debug().Str("user_id", userID).Msg("GetProfile: returning user profile")
	c.JSON(http.StatusOK, gin.H{
		"id":      user.ID,
		"email":   user.Email,
		"name":    user.Name,
		"picture": user.Picture,
	})
}
