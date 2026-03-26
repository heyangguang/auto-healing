package provider

import (
	"strings"

	"github.com/company/auto-healing/internal/model"
)

type secretFieldExtractor func(string) string

func buildMappedSecret(authType string, mapping model.FieldMapping, extractor secretFieldExtractor) (*model.Secret, error) {
	secret := &model.Secret{
		AuthType: authType,
		Username: extractMappedField(extractor, mapping.Username, "username", "root"),
	}
	switch authType {
	case "ssh_key":
		secret.PrivateKey = extractMappedField(extractor, mapping.PrivateKey, "private_key", "")
	case "password":
		secret.Password = extractMappedField(extractor, mapping.Password, "password", "")
	}
	if secret.PrivateKey == "" && secret.Password == "" {
		return nil, ErrSecretNotFound
	}
	return secret, nil
}

func extractMappedField(extractor secretFieldExtractor, configuredPath, defaultPath, fallback string) string {
	path := configuredPath
	if path == "" {
		path = defaultPath
	}
	if value := extractor(path); value != "" {
		return value
	}
	return fallback
}

func extractStringPath(data map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := data
	for i, part := range parts {
		if i == len(parts)-1 {
			if value, ok := current[part].(string); ok {
				return value
			}
			return ""
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}
