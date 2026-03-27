package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/company/auto-healing/internal/model"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	"github.com/company/auto-healing/internal/pkg/response"
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
	case http.StatusBadGateway:
		response.Error(c, http.StatusBadGateway, response.CodeInternal, message)
	default:
		respondInternalError(c, "SECRETS", internalMessage, err)
	}
}

func writeSecretQueryError(c *gin.Context, err error) {
	status, message, ok := classifySecretQueryError(err)
	if !ok {
		respondInternalError(c, "SECRETS", "查询密钥失败", err)
		return
	}
	writeSecretsError(c, status, message)
}

func classifySourceAdminError(err error) (int, string) {
	switch {
	case errors.Is(err, secretsSvc.ErrSecretsSourceInvalidID):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceInvalidInput):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsProviderInvalidConfig):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, secretsSvc.ErrDefaultSourceMustBeActive):
		return http.StatusBadRequest, "默认密钥源必须为启用状态"
	case errors.Is(err, secretsSvc.ErrSecretsSourceInUse):
		return http.StatusConflict, err.Error()
	case isDuplicateConstraintError(err):
		return http.StatusConflict, "密钥源名称已存在"
	case errors.Is(err, secretsSvc.ErrSecretsSourceAlreadyActive), errors.Is(err, secretsSvc.ErrSecretsSourceAlreadyInactive):
		return http.StatusConflict, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsSourceInactive), errors.Is(err, secretsSvc.ErrDefaultSecretsSourceUnavailable):
		return http.StatusConflict, err.Error()
	case errors.Is(err, secretsSvc.ErrSecretsProviderConnectionFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderAuthFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderRequestFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderInvalidResponse):
		return http.StatusBadGateway, "密钥提供方不可用"
	case errors.Is(err, secretsSvc.ErrSecretsSourceNotFound):
		return http.StatusNotFound, "密钥源不存在"
	default:
		return 0, ""
	}
}

func isDuplicateConstraintError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicated key") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique violation")
}

func writeSourceProbeError(c *gin.Context, err error) {
	status, message, ok := classifySourceProbeError(err)
	if !ok {
		respondInternalError(c, "SECRETS", "连接测试失败", err)
		return
	}
	writeSecretsError(c, status, message)
}

func publicSecretQueryErrorMessage(err error) string {
	_, message, ok := classifySecretQueryError(err)
	if ok {
		return message
	}
	return "查询密钥失败"
}

func classifySecretQueryError(err error) (int, string, bool) {
	switch {
	case errors.Is(err, secretsSvc.ErrSecretsSourceInvalidID):
		return http.StatusBadRequest, err.Error(), true
	case errors.Is(err, secretsSvc.ErrSecretsQueryTargetRequired):
		return http.StatusBadRequest, "请提供 hostname 或 ip_address", true
	case errors.Is(err, secretsSvc.ErrSecretsSourceInactive), errors.Is(err, secretsSvc.ErrDefaultSecretsSourceUnavailable):
		return http.StatusConflict, err.Error(), true
	case errors.Is(err, secretsSvc.ErrSecretsSourceNotFound):
		return http.StatusNotFound, "密钥源不存在", true
	case errors.Is(err, secretsSvc.ErrSecretNotFound):
		return http.StatusNotFound, "密钥未找到", true
	case errors.Is(err, secretsSvc.ErrSecretsProviderInvalidConfig):
		return http.StatusBadRequest, "密钥源配置无效", true
	case errors.Is(err, secretsSvc.ErrSecretsProviderConnectionFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderAuthFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderRequestFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderInvalidResponse):
		return http.StatusBadGateway, "密钥提供方不可用", true
	default:
		return 0, "", false
	}
}

func classifySourceProbeError(err error) (int, string, bool) {
	switch {
	case errors.Is(err, secretsSvc.ErrSecretsSourceInvalidID):
		return http.StatusBadRequest, err.Error(), true
	case errors.Is(err, secretsSvc.ErrSecretsSourceNotFound):
		return http.StatusNotFound, "密钥源不存在", true
	case errors.Is(err, secretsSvc.ErrSecretsProviderInvalidConfig):
		return http.StatusBadRequest, "密钥源配置无效", true
	case errors.Is(err, secretsSvc.ErrSecretsProviderConnectionFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderAuthFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderRequestFailed),
		errors.Is(err, secretsSvc.ErrSecretsProviderInvalidResponse):
		return http.StatusBadGateway, "连接测试失败，请检查提供方状态和认证配置", true
	default:
		return 0, "", false
	}
}

func writeSecretsError(c *gin.Context, status int, message string) {
	switch status {
	case http.StatusBadRequest:
		response.BadRequest(c, message)
	case http.StatusConflict:
		response.Conflict(c, message)
	case http.StatusNotFound:
		response.NotFound(c, message)
	case http.StatusBadGateway:
		response.Error(c, http.StatusBadGateway, response.CodeInternal, message)
	default:
		response.InternalError(c, message)
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
	case "token", "password", "secret", "api_key", "private_key", "secret_id", "passphrase", "access_token", "client_token", "authorization":
		return true
	default:
		return false
	}
}
