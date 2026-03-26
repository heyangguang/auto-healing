package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DictionaryHandler 字典值处理器
type DictionaryHandler struct {
	svc *service.DictionaryService
}

type updateDictionaryRequest struct {
	Label     *string     `json:"label"`
	LabelEn   *string     `json:"label_en"`
	Color     *string     `json:"color"`
	TagColor  *string     `json:"tag_color"`
	Badge     *string     `json:"badge"`
	Icon      *string     `json:"icon"`
	Bg        *string     `json:"bg"`
	Extra     *model.JSON `json:"extra"`
	SortOrder *int        `json:"sort_order"`
	IsActive  *bool       `json:"is_active"`
}

// NewDictionaryHandler 创建处理器
func NewDictionaryHandler() *DictionaryHandler {
	svc := service.NewDictionaryService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.LoadCache(ctx); err != nil {
		panic(fmt.Errorf("初始化字典缓存失败: %w", err))
	}
	return &DictionaryHandler{svc: svc}
}

// ListDictionaries 批量查询字典
// GET /api/v1/dictionaries?types=instance_status,node_type&active_only=true
func (h *DictionaryHandler) ListDictionaries(c *gin.Context) {
	types := parseDictionaryTypes(c.Query("types"))
	activeOnly, err := parseDictionaryActiveOnly(c.DefaultQuery("active_only", "true"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	data, err := h.svc.GetAll(c.Request.Context(), types, activeOnly)
	if err != nil {
		respondInternalError(c, "DICT", "查询字典失败", err)
		return
	}

	// 统计
	totalItems := 0
	for _, items := range data {
		totalItems += len(items)
	}

	response.Success(c, gin.H{
		"items": data,
		"meta": gin.H{
			"types_count": len(data),
			"items_count": totalItems,
		},
	})
}

// ListTypes 查询可用类型列表
// GET /api/v1/dictionaries/types
func (h *DictionaryHandler) ListTypes(c *gin.Context) {
	types, err := h.svc.GetTypes(c.Request.Context())
	if err != nil {
		respondInternalError(c, "DICT", "查询字典类型失败", err)
		return
	}

	response.Success(c, gin.H{
		"items": types,
		"total": len(types),
	})
}

// CreateDictionary 创建字典项
// POST /api/v1/dictionaries
func (h *DictionaryHandler) CreateDictionary(c *gin.Context) {
	var item model.Dictionary
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	if item.DictType == "" || item.DictKey == "" || item.Label == "" {
		response.BadRequest(c, "dict_type, dict_key, label 为必填项")
		return
	}

	item.ID = uuid.New()
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	// 平台接口创建的是可维护的业务字典项，不允许借此创建系统内置项。
	item.IsSystem = false

	if err := h.svc.Create(c.Request.Context(), &item); err != nil {
		respondInternalError(c, "DICT", "创建字典项失败", err)
		return
	}

	response.Created(c, item)
}

// UpdateDictionary 更新字典项
// PUT /api/v1/dictionaries/:id
func (h *DictionaryHandler) UpdateDictionary(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondDictionaryLookupError(c, err)
		return
	}

	var input updateDictionaryRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	applyDictionaryPatch(existing, &input)
	existing.UpdatedAt = time.Now()

	if err := h.svc.Update(c.Request.Context(), existing); err != nil {
		respondInternalError(c, "DICT", "更新字典项失败", err)
		return
	}

	response.Success(c, existing)
}

// DeleteDictionary 删除字典项
// DELETE /api/v1/dictionaries/:id
func (h *DictionaryHandler) DeleteDictionary(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		respondDictionaryLookupError(c, err)
		return
	}

	if existing.IsSystem {
		response.Forbidden(c, "系统内置字典项不可删除")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondInternalError(c, "DICT", "删除字典项失败", err)
		return
	}

	response.Message(c, "删除成功")
}

// GetDictionaryService 返回服务实例（供路由外部调用 Seed）
func (h *DictionaryHandler) GetDictionaryService() *service.DictionaryService {
	return h.svc
}

func applyDictionaryPatch(existing *model.Dictionary, input *updateDictionaryRequest) {
	if input.Label != nil {
		existing.Label = *input.Label
	}
	if input.LabelEn != nil {
		existing.LabelEn = *input.LabelEn
	}
	if input.Color != nil {
		existing.Color = *input.Color
	}
	if input.TagColor != nil {
		existing.TagColor = *input.TagColor
	}
	if input.Badge != nil {
		existing.Badge = *input.Badge
	}
	if input.Icon != nil {
		existing.Icon = *input.Icon
	}
	if input.Bg != nil {
		existing.Bg = *input.Bg
	}
	if input.Extra != nil {
		existing.Extra = *input.Extra
	}
	if input.SortOrder != nil {
		existing.SortOrder = *input.SortOrder
	}
	if input.IsActive != nil {
		existing.IsActive = *input.IsActive
	}
}

func parseDictionaryTypes(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	types := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			types = append(types, trimmed)
		}
	}
	return types
}

func parseDictionaryActiveOnly(raw string) (bool, error) {
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errors.New("active_only 必须是合法布尔值")
	}
	return parsed, nil
}

func respondDictionaryLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.NotFound(c, "字典项不存在")
		return
	}
	respondInternalError(c, "DICT", "查询字典项失败", err)
}
