package secrets

import "errors"

var (
	ErrSecretsSourceNotFound           = errors.New("secrets source not found")
	ErrSecretsSourceInvalidID          = errors.New("invalid secrets source id")
	ErrSecretsSourceInvalidInput       = errors.New("invalid secrets source input")
	ErrSecretsSourceInactive           = errors.New("secrets source inactive")
	ErrDefaultSecretsSourceUnavailable = errors.New("default secrets source unavailable")
	ErrSecretsSourceInUse              = errors.New("secrets source in use")
)
