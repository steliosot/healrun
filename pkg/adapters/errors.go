package adapters

import (
	"errors"
)

var (
	ErrProviderNotAvailable = errors.New("model provider not available")
	ErrNoAPIKey             = errors.New("API key not configured")
)
