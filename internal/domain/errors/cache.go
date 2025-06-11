package errors

import "errors"

var (
	ErrKeyNotFound          = errors.New("key not found")
	ErrInvalidKey           = errors.New("invalid key")
	ErrInvalidValue         = errors.New("invalid value")
	ErrInvalidConfiguration = errors.New("invalid configuration")
	ErrNoNodesAvailable     = errors.New("no nodes available")
	ErrClientNotFound       = errors.New("client not found for node")
	ErrCacheUnavailable     = errors.New("cache unavailable")
)
