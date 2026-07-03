package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/darknessnerd/filebot-webui/internal/domain"
	"github.com/darknessnerd/filebot-webui/internal/repository"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	require.NoError(t, repository.RunMigrations(db))
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openTestDB(t)
	// second run must not fail (CREATE TABLE IF NOT EXISTS)
	assert.NoError(t, repository.RunMigrations(db))
}

func TestUpsert_NewUser_AssignsID(t *testing.T) {
	repo := repository.NewUserRepository(openTestDB(t))
	u := &domain.User{PlexID: "p1", PlexUsername: "alice", PlexEmail: "a@x.com", PlexToken: "tok"}

	saved, err := repo.Upsert(context.Background(), u)
	require.NoError(t, err)

	assert.Greater(t, saved.ID, 0)
	assert.Equal(t, "p1", saved.PlexID)
	assert.Equal(t, "alice", saved.PlexUsername)
	assert.Equal(t, "tok", saved.PlexToken)
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestUpsert_ExistingUser_UpdatesFields(t *testing.T) {
	repo := repository.NewUserRepository(openTestDB(t))
	ctx := context.Background()

	first, err := repo.Upsert(ctx, &domain.User{PlexID: "p2", PlexUsername: "bob", PlexToken: "old"})
	require.NoError(t, err)

	updated, err := repo.Upsert(ctx, &domain.User{PlexID: "p2", PlexUsername: "bob2", PlexToken: "new"})
	require.NoError(t, err)

	assert.Equal(t, first.ID, updated.ID, "ID must not change on upsert")
	assert.Equal(t, "bob2", updated.PlexUsername)
	assert.Equal(t, "new", updated.PlexToken)
}

func TestFindByPlexID_Found(t *testing.T) {
	repo := repository.NewUserRepository(openTestDB(t))
	ctx := context.Background()

	_, err := repo.Upsert(ctx, &domain.User{PlexID: "p3", PlexUsername: "carol"})
	require.NoError(t, err)

	u, err := repo.FindByPlexID(ctx, "p3")
	require.NoError(t, err)
	assert.Equal(t, "carol", u.PlexUsername)
}

func TestFindByPlexID_NotFound_ReturnsErrNotFound(t *testing.T) {
	repo := repository.NewUserRepository(openTestDB(t))

	_, err := repo.FindByPlexID(context.Background(), "ghost")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}
