package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

// userStore is the interface this service needs from the repository layer.
// Defined here (consuming package), not in repository.
type userStore interface {
	FindByPlexID(ctx context.Context, plexID string) (*domain.User, error)
	Upsert(ctx context.Context, u *domain.User) (*domain.User, error)
}

// Service implements Plex PIN auth + JWT management.
type Service struct {
	store        userStore
	jwtSecret    []byte
	jwtExpiresIn time.Duration
	jwtIssuer    string
	plexClientID string
	log          logger.Logger
}

func New(store userStore, jwtSecret string, jwtExpiresIn time.Duration, jwtIssuer, plexClientID string, log logger.Logger) *Service {
	return &Service{
		store:        store,
		jwtSecret:    []byte(jwtSecret),
		jwtExpiresIn: jwtExpiresIn,
		jwtIssuer:    jwtIssuer,
		plexClientID: plexClientID,
		log:          log,
	}
}

// StartPlexPIN creates a Plex PIN and returns the auth app URL plus PIN credentials.
func (s *Service) StartPlexPIN(ctx context.Context, forwardURL string) (authAppURL string, pinID int64, pinCode string, err error) {
	pinID, pinCode, err = createPlexPin(s.plexClientID, s.plexClientID)
	if err != nil {
		return "", 0, "", fmt.Errorf("auth.StartPlexPIN: %w", err)
	}
	authAppURL = plexAuthAppURL(s.plexClientID, pinCode, s.plexClientID, forwardURL)
	s.log.Info().Int64("pin_id", pinID).Msg("plex PIN created")
	return authAppURL, pinID, pinCode, nil
}

// CompleteAuth polls Plex for the claimed PIN, upserts the user, and returns the user.
func (s *Service) CompleteAuth(ctx context.Context, pinID int64, pinCode string) (*domain.User, error) {
	token, err := pollPlexPin(pinID, pinCode, s.plexClientID, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("auth.CompleteAuth poll: %w", err)
	}

	info, err := getPlexUserInfo(s.plexClientID, s.plexClientID, token)
	if err != nil {
		return nil, fmt.Errorf("auth.CompleteAuth userinfo: %w", err)
	}

	u := &domain.User{
		PlexID:       info.id,
		PlexUsername: info.username,
		PlexEmail:    info.email,
		PlexAvatar:   info.avatar,
		PlexToken:    token,
	}
	saved, err := s.store.Upsert(ctx, u)
	if err != nil {
		return nil, err
	}
	s.log.Info().Str("plex_username", saved.PlexUsername).Str("plex_id", saved.PlexID).Msg("user authenticated")
	return saved, nil
}

// IssueJWT signs a JWT for the given user.
func (s *Service) IssueJWT(u *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub": u.PlexID,
		"exp": time.Now().Add(s.jwtExpiresIn).Unix(),
		"iat": time.Now().Unix(),
		"iss": s.jwtIssuer,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("auth.IssueJWT: %w", err)
	}
	s.log.Debug().Str("plex_id", u.PlexID).Msg("JWT issued")
	return signed, nil
}

// ValidateJWT parses and validates a JWT, returning the plexID (sub claim).
func (s *Service) ValidateJWT(tokenStr string) (plexID string, err error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithIssuer(s.jwtIssuer))
	if err != nil {
		return "", fmt.Errorf("auth.ValidateJWT: %w", err)
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || !t.Valid {
		return "", fmt.Errorf("auth.ValidateJWT: invalid token")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", fmt.Errorf("auth.ValidateJWT: missing sub claim")
	}
	s.log.Debug().Str("plex_id", sub).Msg("JWT validated")
	return sub, nil
}

// --- Plex API helpers (package-private, not methods) ---

type plexUserInfo struct {
	id       string
	username string
	email    string
	avatar   string
}

func createPlexPin(clientIdentifier, appName string) (pinID int64, pinCode string, err error) {
	form := url.Values{}
	form.Set("strong", "true")
	form.Set("X-Plex-Product", appName)
	form.Set("X-Plex-Client-Identifier", clientIdentifier)

	req, err := http.NewRequest(http.MethodPost, "https://plex.tv/api/v2/pins", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, "", fmt.Errorf("createPlexPin: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("createPlexPin: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, "", fmt.Errorf("createPlexPin decode: %w", err)
	}
	return result.ID, result.Code, nil
}

func plexAuthAppURL(clientIdentifier, pinCode, appName, forwardURL string) string {
	params := fmt.Sprintf(
		"clientID=%s&code=%s&context%%5Bdevice%%5D%%5Bproduct%%5D=%s&forwardUrl=%s",
		clientIdentifier, pinCode, url.QueryEscape(appName), url.QueryEscape(forwardURL),
	)
	return "https://app.plex.tv/auth#?" + params
}

func pollPlexPin(pinID int64, pinCode, clientIdentifier string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	endpoint := fmt.Sprintf("https://plex.tv/api/v2/pins/%d", pinID)

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return "", fmt.Errorf("pollPlexPin: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("code", pinCode)
		req.Header.Set("X-Plex-Client-Identifier", clientIdentifier)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("pollPlexPin: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			AuthToken string `json:"authToken"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("pollPlexPin decode: %w", err)
		}
		if result.AuthToken != "" {
			return result.AuthToken, nil
		}
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("pollPlexPin: PIN expired or not claimed within timeout")
}

func getPlexUserInfo(clientIdentifier, appName, accessToken string) (*plexUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, "https://plex.tv/api/v2/user", nil)
	if err != nil {
		return nil, fmt.Errorf("getPlexUserInfo: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Product", appName)
	req.Header.Set("X-Plex-Client-Identifier", clientIdentifier)
	req.Header.Set("X-Plex-Token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getPlexUserInfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getPlexUserInfo: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("getPlexUserInfo read: %w", err)
	}

	// Plex returns id as either a number or string depending on API version.
	var raw struct {
		ID       any    `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Thumb    string `json:"thumb"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("getPlexUserInfo decode: %w", err)
	}

	var id string
	switch v := raw.ID.(type) {
	case float64:
		id = fmt.Sprintf("%.0f", v)
	case string:
		id = v
	default:
		id = fmt.Sprintf("%v", v)
	}

	return &plexUserInfo{
		id:       id,
		username: raw.Username,
		email:    raw.Email,
		avatar:   raw.Thumb,
	}, nil
}
