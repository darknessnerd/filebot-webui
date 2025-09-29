package handlers

import (
	"errors"
	"testing"
	"webui-skeleton/internal/models"
	_ "webui-skeleton/internal/repository"
)

type mockPlexServerRepository struct {
	preferredServer    *models.PlexServer
	preferredServerErr error
}

func (m *mockPlexServerRepository) GetPreferredPlexServer(userID int) (*models.PlexServer, error) {
	return m.preferredServer, m.preferredServerErr
}

func (m *mockPlexServerRepository) SetPreferredPlexServer(userID int, serverID int) error {
	return nil
}

func (m *mockPlexServerRepository) BatchUpsertAndFetchServers(user *models.User, servers []models.PlexServer) ([]models.PlexServer, error) {
	return nil, nil
}

func (m *mockPlexServerRepository) GetServersByUser(userID int) ([]models.PlexServer, error) {
	return nil, nil
}

func TestGetWorkingURLServer_LocalConnection(t *testing.T) {
	h := &PlexHandler{repo: &mockPlexServerRepository{
		preferredServer: &models.PlexServer{
			Connections: []models.PlexServerConnection{
				{URI: "http://192.168.1.10:32400", Local: true},
				{URI: "http://1.2.3.4:32400", Local: false},
			},
		},
	}}
	user := &models.User{ID: 1, PlexUsername: "testuser"}
	clientIP := "192.168.1.20"
	url, err := h.getWorkingURLServer(user, clientIP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "http://192.168.1.10:32400" {
		t.Errorf("expected local connection, got %s", url)
	}
}

func TestGetWorkingURLServer_ExternalConnection(t *testing.T) {
	h := &PlexHandler{repo: &mockPlexServerRepository{
		preferredServer: &models.PlexServer{
			Connections: []models.PlexServerConnection{
				{URI: "http://1.2.3.4:32400", Local: false},
			},
		},
	}}
	user := &models.User{ID: 1, PlexUsername: "testuser"}
	clientIP := "192.168.1.20"
	url, err := h.getWorkingURLServer(user, clientIP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "http://1.2.3.4:32400" {
		t.Errorf("expected external connection, got %s", url)
	}
}

func TestGetWorkingURLServer_NoConnection(t *testing.T) {
	h := &PlexHandler{repo: &mockPlexServerRepository{
		preferredServer: &models.PlexServer{
			Connections: []models.PlexServerConnection{},
		},
	}}
	user := &models.User{ID: 1, PlexUsername: "testuser"}
	clientIP := "192.168.1.20"
	url, err := h.getWorkingURLServer(user, clientIP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty string, got %s", url)
	}
}

func TestGetWorkingURLServer_RepoError(t *testing.T) {
	h := &PlexHandler{repo: &mockPlexServerRepository{
		preferredServerErr: errors.New("Repo error"),
	}}
	user := &models.User{ID: 1, PlexUsername: "testuser"}
	clientIP := "192.168.1.20"
	url, err := h.getWorkingURLServer(user, clientIP)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if url != "" {
		t.Errorf("expected empty string, got %s", url)
	}
}
