package errors

import "errors"

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrInvalidKey       = errors.New("invalid key")
	ErrInvalidValue     = errors.New("invalid value")
	ErrCacheUnavailable = errors.New("cache unavailable")
)
