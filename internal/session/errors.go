package session

import "errors"

var (
	ErrInvalidState = errors.New("invalid runtime state")
	ErrUnavailable  = errors.New("runtime dependency unavailable")
)
