package repository

import (
	"database/sql"
	"fmt"
	"time"
	"webui-skeleton/internal/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByPlexOrEmail(plexID, email string) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(`
		SELECT id, plex_id, email, plex_email, name, picture, plex_username, plex_avatar, plex_token, created_at, updated_at
		FROM users WHERE plex_id = ? OR email = ? OR plex_email = ?`,
		plexID, email, email).Scan(
		&user.ID, &user.PlexID, &user.Email, &user.PlexEmail, &user.Name, &user.Picture, &user.PlexUsername, &user.PlexAvatar, &user.PlexToken, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) CreatePlexUser(user *models.User) (*models.User, error) {
	result, err := r.db.Exec(`
		INSERT INTO users (plex_id, email, name, picture, plex_username, plex_email, plex_avatar, plex_token) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		user.PlexID, user.Email, user.Name, user.Picture, user.PlexUsername, user.PlexEmail, user.PlexAvatar, user.PlexToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	userID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %w", err)
	}
	user.ID = int(userID)
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	return user, nil
}

func (r *UserRepository) UpdatePlexUser(user *models.User) error {
	_, err := r.db.Exec(`
		UPDATE users SET plex_id = ?, name = ?, picture = ?, plex_username = ?, plex_email = ?, plex_avatar = ?, plex_token = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`,
		user.PlexID, user.Name, user.Picture, user.PlexUsername, user.PlexEmail, user.PlexAvatar, user.PlexToken, user.ID)
	return err
}

func (r *UserRepository) FindByIdentifier(identifier string) (*models.User, error) {
	var user models.User
	var plexID, email, name, picture, plexUsername, plexEmail, plexAvatar, plexToken sql.NullString
	row := r.db.QueryRow(`SELECT id, plex_id, email, name, picture, created_at, updated_at, plex_username, plex_email, plex_avatar, plex_token FROM users WHERE plex_id = ? OR plex_email = ? OR email = ? LIMIT 1`, identifier, identifier, identifier)
	if err := row.Scan(&user.ID, &plexID, &email, &name, &picture, &user.CreatedAt, &user.UpdatedAt, &plexUsername, &plexEmail, &plexAvatar, &plexToken); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	user.PlexID = ""
	if plexID.Valid {
		user.PlexID = plexID.String
	}
	user.Email = ""
	if email.Valid {
		user.Email = email.String
	}
	user.Name = ""
	if name.Valid {
		user.Name = name.String
	}
	user.Picture = ""
	if picture.Valid {
		user.Picture = picture.String
	}
	user.PlexUsername = ""
	if plexUsername.Valid {
		user.PlexUsername = plexUsername.String
	}
	user.PlexEmail = ""
	if plexEmail.Valid {
		user.PlexEmail = plexEmail.String
	}
	user.PlexAvatar = ""
	if plexAvatar.Valid {
		user.PlexAvatar = plexAvatar.String
	}
	user.PlexToken = ""
	if plexToken.Valid {
		user.PlexToken = plexToken.String
	}
	return &user, nil
}

func (r *UserRepository) UpdatePlexToken(userID int, plexToken string) error {
	_, err := r.db.Exec(`UPDATE users SET plex_token = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, plexToken, userID)
	return err
}
