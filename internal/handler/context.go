package handler

import (
	"context"

	"github.com/darknessnerd/filebot-webui/internal/domain"
)

type ctxKey int

const userCtxKey ctxKey = iota

func WithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

func UserFromContext(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(userCtxKey).(*domain.User)
	return u, ok
}
