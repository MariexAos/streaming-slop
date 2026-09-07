package postgres

import "errors"

var (
	ErrNotFound      = errors.New("postgres record not found")
	ErrConflict      = errors.New("postgres version conflict")
	ErrActiveAttempt = errors.New("segment already has an active attempt")
)
