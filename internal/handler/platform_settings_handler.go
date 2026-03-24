package handler

import (
	"sort"
	"strconv"
	"strings"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ==================== 平台级设置 Handler ====================
// 平台管理员管理全局配置，与租户无关

// PlatformSettingsHandler 平台设置处理器
type PlatformSettingsHandler struct {
	repo *repository.PlatformSettingsRepository
}

// NewPlatformSettingsHandler 创建平台设置处理器
func NewPlatformSettingsHandler() *PlatformSettingsHandler {
	return &PlatformSettingsHandler{
		repo: repository.NewPlatformSettingsRepository(),
	}
}

// ==================== DTO ====================

// updatePlatformSettingRequest 更新平台设置请求
type updatePlatformSettingRequest struct {
	Value string `json:"value" binding:"required"`
}

// platformSettingsGroupResponse 按模块分组的设置响应
type platformSettingsGroupResponse struct {
	Module   string                  `json:"module"`
	Settings []model.PlatformSetting `json:"settings"`
}

func isSensitiveSettingKey(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "api_key")
}

// ==================== Handlers ====================

// ListSettings 获取所有平台设置（按 module 分组）
// GET /api/v1/platform/settings?module=site_message
func (h *PlatformSettingsHandler) ListSettings(c *gin.Context) {
	module := c.Query("module")

	var settings []model.PlatformSetting
	var err error

	if module != "" {
		settings, err = h.repo.GetByModule(c.Request.Context(), module)
	} else {
		settings, err = h.repo.GetAll(c.Request.Context())
	}
	if err != nil {
		response.InternalError(c, "获取平台设置失败")
		return
	}

	// 按 module 分组
	grouped := make(map[string][]model.PlatformSetting)
	for _, s := range settings {
		grouped[s.Module] = append(grouped[s.Module], s)
	}

	modules := make([]string, 0, len(grouped))
	for mod := range grouped {
		modules = append(modules, mod)
	}
	sort.Strings(modules)

	var result []platformSettingsGroupResponse
	for _, mod := range modules {
		items := grouped[mod]
		for i := range items {
			if isSensitiveSettingKey(items[i].Key) && items[i].Value != "" {
				items[i].Value = "********"
			}
		}
		result = append(result, platformSettingsGroupResponse{
			Module:   mod,
			Settings: items,
		})
	}

	response.Success(c, result)
}

// UpdateSetting 修改单个设置
// PUT /api/v1/platform/settings/:key
func (h *PlatformSettingsHandler) UpdateSetting(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "设置 key 不能为空")
		return
	}

	var req updatePlatformSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误：value 为必填")
		return
	}

	// 获取当前设置，用于类型校验
	existing, err := h.repo.GetByKey(c.Request.Context(), key)
	if err != nil {
		response.NotFound(c, "设置不存在: "+key)
		return
	}

	// 类型校验
	if err := validateSettingValue(existing.Type, req.Value); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 获取操作人
	var updatedBy *uuid.UUID
	if userIDStr := middleware.GetUserID(c); userIDStr != "" {
		if uid, parseErr := uuid.Parse(userIDStr); parseErr == nil {
			updatedBy = &uid
		}
	}

	updated, err := h.repo.Update(c.Request.Context(), key, req.Value, updatedBy)
	if err != nil {
		response.InternalError(c, "更新设置失败")
		return
	}
	if isSensitiveSettingKey(updated.Key) && updated.Value != "" {
		updated.Value = "********"
	}

	response.Success(c, updated)
}

// validateSettingValue 根据类型验证设置值
func validateSettingValue(settingType, value string) error {
	switch settingType {
	case model.SettingTypeInt:
		v, err := strconv.Atoi(value)
		if err != nil {
			return &settingValidationError{"值必须是整数"}
		}
		// 通用整数范围限制
		if v < 0 || v > 999999 {
			return &settingValidationError{"整数值必须在 0-999999 范围内"}
		}
	case model.SettingTypeBool:
		if value != "true" && value != "false" {
			return &settingValidationError{"布尔值必须为 true 或 false"}
		}
	case model.SettingTypeJSON:
		// JSON 格式，暂不做严格校验
	case model.SettingTypeString:
		// 字符串无限制
	}
	return nil
}

// settingValidationError 设置校验错误
type settingValidationError struct {
	msg string
}

func (e *settingValidationError) Error() string {
	return e.msg
}
