package domain

import "time"

type User struct {
	ID           int
	PlexID       string
	PlexUsername string
	PlexEmail    string
	PlexAvatar   string
	PlexToken    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
