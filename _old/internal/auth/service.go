package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	repository2 "webui-skeleton/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"webui-skeleton/internal/logger"
	"webui-skeleton/internal/models"
)

type Service struct {
	db           *sql.DB
	UserRepo     *repository2.UserRepository
	jwtSecret    []byte
	jwtExpiresIn time.Duration
	jwtIssuer    string
	plexConfig   oauth2.Config
}

// NewService creates a new authentication service
func NewService(db *sql.DB, jwtSecret string, jwtExpiresIn time.Duration, jwtIssuer string,
	plexClientID, plexClientSecret, plexRedirectURL string) *Service {

	plexConfig := oauth2.Config{
		ClientID:     plexClientID,
		ClientSecret: plexClientSecret,
		RedirectURL:  plexRedirectURL,
		Scopes:       []string{""}, // Plex does not require scopes
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://plex.tv/auth#?client_id=" + plexClientID,
			TokenURL: "https://plex.tv/api/v2/oauth/token",
		},
	}

	return &Service{
		db:           db,
		UserRepo:     repository2.NewUserRepository(db),
		jwtSecret:    []byte(jwtSecret),
		jwtExpiresIn: jwtExpiresIn,
		jwtIssuer:    jwtIssuer,
		plexConfig:   plexConfig,
	}
}

// GenerateJWT generates a JWT token for a user
func (s *Service) GenerateJWT(id string, email string, name string, provider string) (string, error) {

	claims := jwt.MapClaims{
		"user_id":  id,
		"email":    email,
		"name":     name,
		"provider": provider,
		"exp":      time.Now().Add(s.jwtExpiresIn).Unix(),
		"iat":      time.Now().Unix(),
		"iss":      s.jwtIssuer,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateJWT validates a JWT token and returns the claims
func (s *Service) ValidateJWT(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDRaw := claims["user_id"]
		var userID string
		switch v := userIDRaw.(type) {
		case string:
			userID = v
		case float64:
			userID = fmt.Sprintf("%.0f", v)
		default:
			return nil, fmt.Errorf("invalid user_id in token")
		}

		email, ok := claims["email"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid email in token")
		}

		name, ok := claims["name"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid name in token")
		}
		provider, ok := claims["provider"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid name in token")
		}

		return &models.JWTClaims{
			UserID:   userID,
			Email:    email,
			Name:     name,
			Provider: provider,
		}, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// getTokenFromRequest extracts JWT token from cookie or Authorization header
func (s *Service) getTokenFromRequest(c *gin.Context) string {
	// First, try to get token from cookie (for web UI)
	if token, err := c.Cookie("auth_token"); err == nil && token != "" {
		return token
	}

	// Second, try to get token from Authorization header (for API)
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}

	return ""
}

func (s *Service) GenerateRandomState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Plex PIN-based authentication

// GenerateClientIdentifier returns a persistent UUID for this app instance
func GenerateClientIdentifier() string {
	id := os.Getenv("PLEX_CLIENT_IDENTIFIER")
	if id != "" {
		return id
	}
	newID := uuid.New().String()
	if err := os.Setenv("PLEX_CLIENT_IDENTIFIER", newID); err != nil {
		return newID // fallback: just return
	}
	return newID
}

// CreatePlexPin creates a new PIN via Plex API
func CreatePlexPin(clientIdentifier, appName string) (pinID int64, pinCode string, err error) {
	form := url.Values{}
	form.Set("strong", "true")
	form.Set("X-Plex-Product", appName)
	form.Set("X-Plex-Client-Identifier", clientIdentifier)

	req, err := http.NewRequest("POST", "https://plex.tv/api/v2/pins", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	var result struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		body, _ := io.ReadAll(resp.Body)
		logger.Log.Error().Err(err).Str("response", string(body)).Msg("Failed to decode Plex PIN response")
		return 0, "", fmt.Errorf("plex pin decode error: %w, body: %s", err, string(body))
	}
	return result.ID, result.Code, nil
}

// GetPlexAuthAppURL constructs the Plex Auth App URL for user authentication
func GetPlexAuthAppURL(clientIdentifier, pinCode, appName, forwardURL string) string {
	params := fmt.Sprintf(
		"clientID=%s&code=%s&context%%5Bdevice%%5D%%5Bproduct%%5D=%s&forwardUrl=%s",
		clientIdentifier, pinCode, url.QueryEscape(appName), url.QueryEscape(forwardURL),
	)
	return "https://app.plex.tv/auth#?" + params
}

// PollPlexPin polls the Plex PIN endpoint until the user claims it and returns the access token
func PollPlexPin(pinID int64, pinCode, clientIdentifier string, timeout time.Duration) (accessToken string, err error) {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		url := fmt.Sprintf("https://plex.tv/api/v2/pins/%d", pinID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("accept", "application/json")
		req.Header.Set("code", pinCode)
		req.Header.Set("X-Plex-Client-Identifier", clientIdentifier)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		responseBody, _ := io.ReadAll(resp.Body)
		logger.Log.Trace().Int64("pin_id", pinID).Str("pin_code", pinCode).Int("status_code", resp.StatusCode).Str("headers", fmt.Sprintf("%v", resp.Header)).Str("body", string(responseBody)).Msg("📬 [Plex] PIN endpoint response")
		logger.Log.Trace().Str("raw_json", string(responseBody)).Msg("🐛 [Plex] Raw JSON response from PIN endpoint")
		var result struct {
			AuthToken string `json:"authToken"`
		}
		if err := json.Unmarshal(responseBody, &result); err != nil {
			logger.Log.Error().Err(err).Str("body", string(responseBody)).Msg("❌ [Plex] Failed to decode PIN response as JSON")
			return "", err
		}
		if result.AuthToken != "" {
			return result.AuthToken, nil
		}
		time.Sleep(1 * time.Second)
	}
	return "", fmt.Errorf("PIN expired or not claimed")
}

// GetPlexUserInfo fetches Plex user info using the access token
func GetPlexUserInfo(clientIdentifier, appName, accessToken string) (*models.PlexUserInfo, error) {
	req, err := http.NewRequest("GET", "https://plex.tv/api/v2/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("X-Plex-Product", appName)
	req.Header.Set("X-Plex-Client-Identifier", clientIdentifier)
	req.Header.Set("X-Plex-Token", accessToken)
	logger.Log.Debug().Msgf("[DEBUG] Plex Token used: %s", accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid access token or error: %d", resp.StatusCode)
	}
	// Read and log raw JSON response
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	logger.Log.Trace().Msgf("[Plex] Raw JSON response: %s", string(bodyBytes))
	var userInfo models.PlexUserInfo
	if err := json.Unmarshal(bodyBytes, &userInfo); err != nil {
		return nil, err
	}
	userInfo.AccessToken = accessToken
	// Convert ID from int to string
	return &userInfo, nil
}

// AuthenticateWithPlex handles the full PIN-based Plex authentication flow
func (s *Service) AuthenticateWithPlex(ctx context.Context, forwardURL string) (authAppURL string, pinID int64, pinCode string, err error) {
	clientIdentifier := GenerateClientIdentifier()
	appName := s.plexConfig.ClientID // Use app name from config
	pinID, pinCode, err = CreatePlexPin(clientIdentifier, appName)
	if err != nil {
		return "", 0, "", err
	}
	authAppURL = GetPlexAuthAppURL(clientIdentifier, pinCode, appName, forwardURL)
	return authAppURL, pinID, pinCode, nil
}

// Convert PlexUserInfo to User model
func PlexUserInfoToUser(plexUserInfo *models.PlexUserInfo) *models.User {
	return &models.User{
		PlexID:       plexUserInfo.ID,
		PlexUsername: plexUserInfo.Username,
		PlexEmail:    plexUserInfo.Email,
		PlexAvatar:   plexUserInfo.Avatar,
		Name:         plexUserInfo.Username,
		Picture:      plexUserInfo.Avatar,
		PlexToken:    plexUserInfo.AccessToken,
		UpdatedAt:    time.Now(),
	}
}

// CompletePlexAuthentication polls for the access token and fetches user info
func (s *Service) CompletePlexAuthentication(ctx context.Context, pinID int64, pinCode string) (*models.User, error) {
	clientIdentifier := GenerateClientIdentifier()
	appName := s.plexConfig.ClientID
	accessToken, err := PollPlexPin(pinID, pinCode, clientIdentifier, 2*time.Minute)
	if err != nil {
		if err.Error() == "invalid character '<' looking for beginning of value" {
			logger.Log.Error().Int64("pin_id", pinID).Str("pin_code", pinCode).Msg("❌ [Plex] Received HTML response from Plex PIN endpoint. Possible expired or invalid PIN. Check network or API status.")
		}
		logger.Log.Error().Err(err).Int64("pin_id", pinID).Str("pin_code", pinCode).Msg("❌ [Plex] Error polling Plex PIN for access token")
		return nil, err
	}
	plexUserInfo, err := GetPlexUserInfo(clientIdentifier, appName, accessToken)
	if err != nil {
		logger.Log.Error().Err(err).Str("access_token", accessToken).Msg("❌ [Plex] Error fetching Plex user info")
		return nil, err
	}

	user, err := s.UserRepo.FindByIdentifier(plexUserInfo.ID)
	if user == nil {
		return s.UserRepo.CreatePlexUser(PlexUserInfoToUser(plexUserInfo))
	}
	if err := s.UserRepo.UpdatePlexUser(user); err != nil {
		logger.Log.Error().Err(err).Msg("❌ [Plex] Failed to update Plex user")
		return nil, fmt.Errorf("failed to update Plex user: %w", err)
	}
	logger.Log.Debug().Str("plex_id", plexUserInfo.ID).Str("plex_username", plexUserInfo.Username).Str("plex_email", plexUserInfo.Email).Msg("✅ [Plex] User updated successfully")
	// Use conversion helper
	updatedUser := PlexUserInfoToUser(plexUserInfo)
	updatedUser.ID = user.ID // preserve existing user ID
	return updatedUser, nil
}

// GetPlexAuthURL returns the Plex OAuth authorization URL
func (s *Service) GetPlexAuthURL(state string) string {
	return s.plexConfig.AuthCodeURL(state)
}

// ExchangePlexCode exchanges the Plex authorization code for an access token
func (s *Service) ExchangePlexCode(ctx context.Context, code string) (string, error) {
	token, err := s.plexConfig.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange Plex code for token: %w", err)
	}

	return token.AccessToken, nil
}

// ValidatePlexToken validates the Plex access token and returns the user ID
func (s *Service) ValidatePlexToken(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://plex.tv/api/v2/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("X-Plex-Token", accessToken)
	logger.Log.Debug().Msgf("[DEBUG] Plex Token used: %s", accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("invalid Plex access token: %d", resp.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// LinkPlexAccount links a Plex account to the user
func (s *Service) LinkPlexAccount(userID int, accessToken string) error {
	req, err := http.NewRequest("GET", "https://plex.tv/api/v2/user", nil)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("X-Plex-Token", accessToken)
	logger.Log.Debug().Msgf("[DEBUG] Plex Token used: %s", accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("invalid Plex access token: %d", resp.StatusCode)
	}
	var plexUserInfo models.PlexUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&plexUserInfo); err != nil {
		return err
	}

	return s.UserRepo.UpdatePlexUser(PlexUserInfoToUser(&plexUserInfo))
}

// RefreshTokenHandler handles the token refresh flow
func (s *Service) RefreshTokenHandler(c *gin.Context) {
	// Get refresh token from request body
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validate the refresh token
	claims, err := s.ValidateJWT(body.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	_, err = s.GenerateJWT(claims.UserID, claims.Email, claims.Name, claims.Provider)
	c.JSON(http.StatusOK, gin.H{"message": "Auth service is running"})
}

// UpdatePlexToken updates the user's Plex token in the database
func (s *Service) UpdatePlexToken(userID int, plexToken string) error {
	return s.UserRepo.UpdatePlexToken(userID, plexToken)
}
