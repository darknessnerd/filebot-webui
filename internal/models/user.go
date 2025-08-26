package models

import (
	"time"
)

type User struct {
	ID           int       `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	Name         string    `json:"name" db:"name"`
	Picture      string    `json:"picture" db:"picture"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	PlexID       string    `json:"plex_id" db:"plex_id"`
	PlexUsername string    `json:"plex_username" db:"plex_username"`
	PlexEmail    string    `json:"plex_email" db:"plex_email"`
	PlexAvatar   string    `json:"plex_avatar" db:"plex_avatar"`
	PlexToken    string    `json:"plex_token" db:"plex_token"`
}
