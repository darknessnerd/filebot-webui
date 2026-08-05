package main

import (
	"context"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/domain"
)

var devUser = &domain.User{
	ID:           1,
	PlexID:       "dev-plex-id",
	PlexUsername: "devuser",
	PlexEmail:    "dev@localhost",
	PlexAvatar:   "",
	PlexToken:    "dev-plex-token",
	CreatedAt:    time.Now(),
	UpdatedAt:    time.Now(),
}

var devTorrents = []domain.Torrent{
	{
		ID:           "abc123",
		Name:         "The.Matrix.1999.1080p.BluRay",
		State:        "Seeding",
		Progress:     100,
		DownloadPath: "/downloads",
		Size:         8_589_934_592,
		IsFinished:   true,
		CompletedOn:  time.Now().Add(-24 * time.Hour),
	},
	{
		ID:           "def456",
		Name:         "Breaking.Bad.S01E01.1080p",
		State:        "Seeding",
		Progress:     100,
		DownloadPath: "/downloads",
		Size:         2_147_483_648,
		IsFinished:   true,
		CompletedOn:  time.Now().Add(-2 * time.Hour),
	},
	{
		ID:           "ghi789",
		Name:         "Dune.Part.Two.2024.2160p.UHD",
		State:        "Seeding",
		Progress:     100,
		DownloadPath: "/downloads",
		Size:         15_032_385_536,
		IsFinished:   true,
		CompletedOn:  time.Now().Add(-10 * time.Minute),
	},
}

type mockDelugeClient struct{}

func (m *mockDelugeClient) ListCompleted(_ context.Context) ([]domain.Torrent, error) {
	return devTorrents, nil
}

func (m *mockDelugeClient) DeleteTorrent(_ context.Context, id string) error {
	return nil
}

type mockPlexClient struct{}

func (m *mockPlexClient) RefreshLibraries(_ context.Context, _ string) error {
	return nil
}
