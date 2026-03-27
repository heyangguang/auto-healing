package httpapi

import (
	"strconv"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/modules/integrations/service/playbook"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PlaybookHandler Playbook 处理器
type PlaybookHandler struct {
	svc *playbook.Service
}

type PlaybookHandlerDeps struct {
	Service *playbook.Service
}

// NewPlaybookHandler 创建 Playbook 处理器
func NewPlaybookHandler() *PlaybookHandler {
	return NewPlaybookHandlerWithDeps(PlaybookHandlerDeps{
		Service: playbook.NewService(),
	})
}

func NewPlaybookHandlerWithDeps(deps PlaybookHandlerDeps) *PlaybookHandler {
	return &PlaybookHandler{
		svc: deps.Service,
	}
}

// ==================== DTO ====================

// CreatePlaybookRequest 创建 Playbook 请求
type CreatePlaybookRequest struct {
	RepositoryID uuid.UUID `json:"repository_id" binding:"required"`
	Name         string    `json:"name" binding:"required"`
	FilePath     string    `json:"file_path" binding:"required"`
	Description  string    `json:"description"`
	ConfigMode   string    `json:"config_mode"` // auto / enhanced，创建时必须指定
}

// UpdatePlaybookRequest 更新 Playbook 请求
type UpdatePlaybookRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (r *UpdatePlaybookRequest) ToUpdateInput() *playbook.UpdateInput {
	return &playbook.UpdateInput{
		Name:        r.Name,
		Description: r.Description,
	}
}

// UpdateVariablesRequest 更新变量请求
type UpdateVariablesRequest struct {
	Variables model.JSONArray `json:"variables" binding:"required"`
}

func buildPlaybookListOptions(c *gin.Context, page, pageSize int) *repository.PlaybookListOptions {
	opts := &repository.PlaybookListOptions{
		Page:       page,
		PageSize:   pageSize,
		Name:       GetStringFilter(c, "name"),
		FilePath:   GetStringFilter(c, "file_path"),
		Status:     c.Query("status"),
		ConfigMode: c.Query("config_mode"),
		SortField:  c.Query("sort_by"),
		SortOrder:  c.Query("sort_order"),
	}
	parsePlaybookListUUIDFilters(c, opts)
	parsePlaybookListBoolFilters(c, opts)
	parsePlaybookListCountFilters(c, opts)
	parsePlaybookListDateFilters(c, opts)
	return opts
}

func parsePlaybookListUUIDFilters(c *gin.Context, opts *repository.PlaybookListOptions) {
	if repoIDStr := c.Query("repository_id"); repoIDStr != "" {
		if id, err := uuid.Parse(repoIDStr); err == nil {
			opts.RepositoryID = &id
		}
	}
}

func parsePlaybookListBoolFilters(c *gin.Context, opts *repository.PlaybookListOptions) {
	if hasVarsStr := c.Query("has_variables"); hasVarsStr != "" {
		hasVars := hasVarsStr == "true"
		opts.HasVariables = &hasVars
	}
	if hasReqStr := c.Query("has_required_vars"); hasReqStr != "" {
		hasReq := hasReqStr == "true"
		opts.HasRequiredVars = &hasReq
	}
}

func parsePlaybookListCountFilters(c *gin.Context, opts *repository.PlaybookListOptions) {
	if minVarsStr := c.Query("min_variables"); minVarsStr != "" {
		if v, err := strconv.Atoi(minVarsStr); err == nil && v >= 0 {
			opts.MinVariables = &v
		}
	}
	if maxVarsStr := c.Query("max_variables"); maxVarsStr != "" {
		if v, err := strconv.Atoi(maxVarsStr); err == nil && v >= 0 {
			opts.MaxVariables = &v
		}
	}
}

func parsePlaybookListDateFilters(c *gin.Context, opts *repository.PlaybookListOptions) {
	if createdFrom := c.Query("created_from"); createdFrom != "" {
		if t, err := time.Parse(time.RFC3339, createdFrom); err == nil {
			opts.CreatedFrom = &t
		}
	}
	if createdTo := c.Query("created_to"); createdTo != "" {
		if t, err := time.Parse(time.RFC3339, createdTo); err == nil {
			opts.CreatedTo = &t
		}
	}
}
