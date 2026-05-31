package repo

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrDuplicate     = errors.New("duplicate")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnavailable   = errors.New("temporarily unavailable")
)
