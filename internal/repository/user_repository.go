package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/darknessnerd/filebot-webui/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByPlexID(ctx context.Context, plexID string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, plex_id, plex_username, plex_email, plex_avatar, plex_token, created_at, updated_at
		FROM users WHERE plex_id = ?`, plexID)

	u := &domain.User{}
	err := row.Scan(&u.ID, &u.PlexID, &u.PlexUsername, &u.PlexEmail, &u.PlexAvatar, &u.PlexToken, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("repository.FindByPlexID: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("repository.FindByPlexID: %w", err)
	}
	return u, nil
}

func (r *UserRepository) Upsert(ctx context.Context, u *domain.User) (*domain.User, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (plex_id, plex_username, plex_email, plex_avatar, plex_token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plex_id) DO UPDATE SET
			plex_username = excluded.plex_username,
			plex_email    = excluded.plex_email,
			plex_avatar   = excluded.plex_avatar,
			plex_token    = excluded.plex_token,
			updated_at    = excluded.updated_at`,
		u.PlexID, u.PlexUsername, u.PlexEmail, u.PlexAvatar, u.PlexToken, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("repository.Upsert: %w", err)
	}
	return r.FindByPlexID(ctx, u.PlexID)
}
