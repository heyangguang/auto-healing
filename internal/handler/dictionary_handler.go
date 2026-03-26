package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DictionaryHandler 字典值处理器
type DictionaryHandler struct {
	svc *service.DictionaryService
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
	typesParam := c.Query("types")
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	var types []string
	if typesParam != "" {
		types = strings.Split(typesParam, ",")
	}

	data, err := h.svc.GetAll(c.Request.Context(), types, activeOnly)
	if err != nil {
		respondInternalError(c, "DICT", "查询字典失败", err)
		return
	}

	response.Success(c, data)
}

// ListTypes 查询可用类型列表
// GET /api/v1/dictionaries/types
func (h *DictionaryHandler) ListTypes(c *gin.Context) {
	types, err := h.svc.GetTypes(c.Request.Context())
	if err != nil {
		respondInternalError(c, "DICT", "查询字典类型失败", err)
		return
	}

	response.Success(c, types)
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
		response.NotFound(c, "字典项不存在")
		return
	}

	var input model.Dictionary
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
		return
	}

	// 更新字段
	existing.Label = input.Label
	existing.LabelEn = input.LabelEn
	existing.Color = input.Color
	existing.TagColor = input.TagColor
	existing.Badge = input.Badge
	existing.Icon = input.Icon
	existing.Bg = input.Bg
	existing.Extra = input.Extra
	existing.SortOrder = input.SortOrder
	existing.IsActive = input.IsActive
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
		response.NotFound(c, "字典项不存在")
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
