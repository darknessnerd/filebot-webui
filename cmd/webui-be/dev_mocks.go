package main

import (
	"context"
	"fmt"
	"strings"
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
	{
		ID:           "ani001",
		Name:         "[SubsPlease] Cowboy Bebop - E01 (1080p) [aid:1]",
		State:        "Seeding",
		Progress:     100,
		DownloadPath: "/downloads",
		Size:         1_610_612_736,
		IsFinished:   true,
		CompletedOn:  time.Now().Add(-45 * time.Minute),
	},
	// regression: episode title after SxxExx + streaming source noise (DSNP, DDP5.1, release group)
	{
		ID:           "fut001",
		Name:         "Futurama.S14E02.Catfish.Hunter.1080p.DSNP.WEB-DL.ENG.ITA.DDP5.1.H264-TheBlackKing",
		State:        "Seeding",
		Progress:     100,
		DownloadPath: "/downloads",
		Size:         2_000_000_000,
		IsFinished:   true,
		CompletedOn:  time.Now().Add(-30 * time.Minute),
	},
	// regression: season folder torrent — individual episode files inside carry NxNN codes.
	// Season year (2026) in folder name != show premiere year (2013); SearchTV retries without year.
	{
		ID:           "ram001",
		Name:         "Rick and Morty - Stagione 09 (2026)",
		State:        "Seeding",
		Progress:     100,
		DownloadPath: "/downloads",
		Size:         5_000_000_000,
		IsFinished:   true,
		CompletedOn:  time.Now().Add(-15 * time.Minute),
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

type mockMetadataResolver struct{}

func (m *mockMetadataResolver) SearchMovie(_ context.Context, query string, year int) (*domain.MovieMatch, error) {
	title := strings.TrimSpace(query)
	if title == "" {
		title = "Mock Movie"
	}
	if year == 0 {
		year = 2024
	}
	return &domain.MovieMatch{ID: 1, Title: title, Year: year}, nil
}

func (m *mockMetadataResolver) SearchTV(_ context.Context, query string, year int) (*domain.TVMatch, error) {
	name := strings.TrimSpace(query)
	if name == "" {
		name = "Mock Show"
	}
	if year == 0 {
		year = 2024
	}
	return &domain.TVMatch{ID: 1, Name: name, Year: year}, nil
}

func (m *mockMetadataResolver) SearchAnime(_ context.Context, query string, year int) (*domain.AnimeMatch, error) {
	title := strings.TrimSpace(query)
	if title == "" {
		title = "Mock Anime"
	}
	if year == 0 {
		year = 1998
	}
	return &domain.AnimeMatch{ID: 1, Title: title, Year: year}, nil
}

func (m *mockMetadataResolver) SearchAnimeByAID(_ context.Context, aid int) (*domain.AnimeMatch, error) {
	if aid <= 0 {
		return nil, fmt.Errorf("invalid aid %d", aid)
	}
	return &domain.AnimeMatch{ID: aid, Title: fmt.Sprintf("Mock Anime AID %d", aid), Year: 1998}, nil
}
