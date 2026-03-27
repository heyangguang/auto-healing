package provider

import (
	"strings"

	secretsmodel "github.com/company/auto-healing/internal/modules/secrets/model"
)

type secretFieldExtractor func(string) string

func validateSecretAuthType(authType string) error {
	switch authType {
	case "ssh_key", "password":
		return nil
	default:
		return newConfigError("auth_type 只支持 ssh_key 或 password")
	}
}

func buildMappedSecret(authType string, mapping secretsmodel.FieldMapping, extractor secretFieldExtractor) (*secretsmodel.Secret, error) {
	secret := &secretsmodel.Secret{
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
		return nil, newInvalidResponseError("提供方响应缺少凭据字段", nil)
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
