package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrInvalidArg        = errors.New("invalid argument")
	ErrFileBotFailed     = errors.New("filebot execution failed")
	ErrDelugeUnavailable = errors.New("deluge unavailable")
)
