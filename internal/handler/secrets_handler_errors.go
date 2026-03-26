package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	secretsSvc "github.com/company/auto-healing/internal/service/secrets"
	"github.com/gin-gonic/gin"
)

func writeSourceAdminError(c *gin.Context, err error, internalMessage string) {
	status, message := classifySourceAdminError(err)
	switch status {
	case http.StatusBadRequest:
		response.BadRequest(c, message)
	case http.StatusConflict:
		response.Conflict(c, message)
	case http.StatusNotFound:
		response.NotFound(c, message)
	default:
		respondInternalError(c, "SECRETS", internalMessage, err)
	}
}

func writeSecretQueryError(c *gin.Context, err error) {
	status, message := classifySecretQueryError(err)
	switch status {
	case http.StatusBadRequest:
		response.BadRequest(c, message)
	case http.StatusConflict:
		response.Conflict(c, message)
	case http.StatusNotFound:
		response.NotFound(c, message)
	default:
		respondInternalError(c, "SECRETS", "查询密钥失败", err)
	}
}

func classifySourceAdminError(err error) (int, string) {
	switch {
	case errors.Is(err, secretsSvc.ErrSecretsSourceInvalidID):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceInUse):
		return http.StatusConflict, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceInactive), errors.Is(err, secretsSvc.ErrDefaultSecretsSourceUnavailable):
		return http.StatusConflict, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceNotFound):
		return http.StatusNotFound, "密钥源不存在"
	default:
		return 0, ""
	}
}

func classifySecretQueryError(err error) (int, string) {
	switch {
	case errors.Is(err, secretsSvc.ErrSecretsSourceInvalidID):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceInactive), errors.Is(err, secretsSvc.ErrDefaultSecretsSourceUnavailable):
		return http.StatusConflict, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceNotFound):
		return http.StatusNotFound, "密钥源不存在"
	default:
		return http.StatusNotFound, "密钥未找到: " + err.Error()
	}
}

// maskConfig 隐藏敏感配置
func maskConfig(config model.JSON) model.JSON {
	masked := make(model.JSON, len(config))
	for key, value := range config {
		masked[key] = maskConfigValue(key, value)
	}
	return masked
}

func maskConfigValue(key string, value interface{}) interface{} {
	if isSensitiveConfigKey(key) {
		return "***"
	}

	switch typed := value.(type) {
	case model.JSON:
		return maskConfig(typed)
	case map[string]interface{}:
		masked := make(map[string]interface{}, len(typed))
		for nestedKey, nestedValue := range typed {
			masked[nestedKey] = maskConfigValue(nestedKey, nestedValue)
		}
		return masked
	case []interface{}:
		masked := make([]interface{}, len(typed))
		for i, item := range typed {
			masked[i] = maskConfigValue("", item)
		}
		return masked
	default:
		return value
	}
}

func isSensitiveConfigKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "token", "password", "secret", "api_key", "private_key", "secret_id", "passphrase":
		return true
	default:
		return false
	}
}
