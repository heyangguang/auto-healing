package secrets

import (
	"errors"

	secretsapi "github.com/company/auto-healing/internal/modules/secrets/providerapi"
)

var (
	ErrSecretsSourceNotFound           = errors.New("secrets source not found")
	ErrSecretsSourceInvalidID          = errors.New("invalid secrets source id")
	ErrSecretsSourceInvalidInput       = errors.New("invalid secrets source input")
	ErrSecretsSourceInactive           = errors.New("secrets source inactive")
	ErrDefaultSecretsSourceUnavailable = errors.New("default secrets source unavailable")
	ErrSecretsSourceInUse              = errors.New("secrets source in use")
	ErrSecretsSourceAlreadyActive      = errors.New("secrets source already active")
	ErrSecretsSourceAlreadyInactive    = errors.New("secrets source already inactive")
	ErrDefaultSourceMustBeActive       = errors.New("default secrets source must be active")
	ErrSecretsQueryTargetRequired      = errors.New("secrets query target required")
	ErrSecretNotFound                  = secretsapi.ErrSecretNotFound
	ErrSecretsProviderConnectionFailed = secretsapi.ErrConnectionFailed
	ErrSecretsProviderAuthFailed       = secretsapi.ErrProviderAuthFailed
	ErrSecretsProviderRequestFailed    = secretsapi.ErrProviderRequestFailed
	ErrSecretsProviderInvalidConfig    = secretsapi.ErrProviderInvalidConfig
	ErrSecretsProviderInvalidResponse  = secretsapi.ErrProviderInvalidResponse
)
