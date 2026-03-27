package httpapi

import (
	"encoding/json"
	"errors"

	"github.com/company/auto-healing/internal/middleware"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PreferenceHandler 用户偏好处理器
type PreferenceHandler struct {
	prefRepo *engagementrepo.UserPreferenceRepository
}

type PreferenceHandlerDeps struct {
	PreferenceRepo *engagementrepo.UserPreferenceRepository
}

func NewPreferenceHandlerWithDeps(deps PreferenceHandlerDeps) *PreferenceHandler {
	return &PreferenceHandler{
		prefRepo: deps.PreferenceRepo,
	}
}

// preferencesRequest 偏好设置请求体
type preferencesRequest struct {
	Preferences json.RawMessage `json:"preferences" binding:"required"`
}

// GetPreferences 获取当前用户的偏好设置
func (h *PreferenceHandler) GetPreferences(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	pref, err := h.prefRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, engagementrepo.ErrPreferenceNotFound) {
			response.Success(c, preferenceResponse{
				UserID:      userID,
				Preferences: json.RawMessage("{}"),
			})
			return
		}
		respondInternalError(c, "PREFERENCE", "获取偏好设置失败", err)
		return
	}

	response.Success(c, pref)
}

// UpdatePreferences 全量更新偏好设置（PUT）
func (h *PreferenceHandler) UpdatePreferences(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req preferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: preferences 字段必填")
		return
	}

	// 验证 preferences 是有效的 JSON 对象
	var check map[string]interface{}
	if err := json.Unmarshal(req.Preferences, &check); err != nil {
		response.BadRequest(c, "preferences 必须是有效的 JSON 对象")
		return
	}
	if check == nil {
		response.BadRequest(c, "preferences 必须是 JSON 对象，不能为 null")
		return
	}

	pref, err := h.prefRepo.Upsert(c.Request.Context(), userID, req.Preferences)
	if err != nil {
		response.InternalError(c, "更新偏好设置失败")
		return
	}

	response.Success(c, pref)
}

// PatchPreferences 部分更新偏好设置（PATCH，合并已有偏好）
func (h *PreferenceHandler) PatchPreferences(c *gin.Context) {
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	var req preferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: preferences 字段必填")
		return
	}
	var patch map[string]interface{}
	if err := json.Unmarshal(req.Preferences, &patch); err != nil || patch == nil {
		response.BadRequest(c, "preferences 必须是 JSON 对象，不能为 null")
		return
	}

	pref, err := h.prefRepo.MergeUpdate(c.Request.Context(), userID, req.Preferences)
	if err != nil {
		respondInternalError(c, "PREFERENCE", "更新偏好设置失败", err)
		return
	}

	response.Success(c, pref)
}
