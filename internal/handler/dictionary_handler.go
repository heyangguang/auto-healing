package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
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
	// 启动时加载缓存
	go svc.LoadCache(context.Background())
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询字典失败: " + err.Error()})
		return
	}

	// 统计
	totalItems := 0
	for _, items := range data {
		totalItems += len(items)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询类型列表失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  types,
		"total": len(types),
	})
}

// CreateDictionary 创建字典项
// POST /api/v1/dictionaries
func (h *DictionaryHandler) CreateDictionary(c *gin.Context) {
	var item model.Dictionary
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
		return
	}

	if item.DictType == "" || item.DictKey == "" || item.Label == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dict_type, dict_key, label 为必填项"})
		return
	}

	item.ID = uuid.New()
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	if err := h.svc.Create(c.Request.Context(), &item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// UpdateDictionary 更新字典项
// PUT /api/v1/dictionaries/:id
func (h *DictionaryHandler) UpdateDictionary(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "字典项不存在"})
		return
	}

	var input model.Dictionary
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": existing})
}

// DeleteDictionary 删除字典项
// DELETE /api/v1/dictionaries/:id
func (h *DictionaryHandler) DeleteDictionary(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "字典项不存在"})
		return
	}

	if existing.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "系统内置字典项不可删除"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetDictionaryService 返回服务实例（供路由外部调用 Seed）
func (h *DictionaryHandler) GetDictionaryService() *service.DictionaryService {
	return h.svc
}
