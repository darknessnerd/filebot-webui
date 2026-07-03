package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/logger"
)

func newTestService(secret string) *Service {
	return New(nil, secret, time.Hour, "filebot-webui", "testapp", logger.New("error", false))
}

func TestIssueAndValidateJWT(t *testing.T) {
	svc := newTestService("supersecret")
	u := &domain.User{PlexID: "plex123"}

	token, err := svc.IssueJWT(u)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	plexID, err := svc.ValidateJWT(token)
	require.NoError(t, err)
	assert.Equal(t, "plex123", plexID)
}

func TestValidateJWT_Expired(t *testing.T) {
	svc := New(nil, "supersecret", -time.Second, "filebot-webui", "testapp", logger.New("error", false))
	u := &domain.User{PlexID: "plex123"}

	token, err := svc.IssueJWT(u)
	require.NoError(t, err)

	_, err = svc.ValidateJWT(token)
	assert.Error(t, err)
}

func TestValidateJWT_WrongKey(t *testing.T) {
	svc := newTestService("supersecret")
	other := newTestService("differentkey")
	u := &domain.User{PlexID: "plex123"}

	token, err := svc.IssueJWT(u)
	require.NoError(t, err)

	_, err = other.ValidateJWT(token)
	assert.Error(t, err)
}

func TestValidateJWT_Tampered(t *testing.T) {
	svc := newTestService("supersecret")
	_, err := svc.ValidateJWT("not.a.token")
	assert.Error(t, err)
}

func TestValidateJWT_MissingSub_ReturnsError(t *testing.T) {
	svc := newTestService("supersecret")
	// Issue token for user with empty PlexID — sub claim will be ""
	u := &domain.User{PlexID: ""}
	token, err := svc.IssueJWT(u)
	require.NoError(t, err)
	_, err = svc.ValidateJWT(token)
	assert.Error(t, err)
}
