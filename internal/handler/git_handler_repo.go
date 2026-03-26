package handler

import (
	"time"

	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListRepos 获取仓库列表
func (h *GitRepoHandler) ListRepos(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	opts := buildGitRepoListOptions(c, page, pageSize)

	repos, total, err := h.svc.ListReposWithOptions(c.Request.Context(), opts)
	if err != nil {
		response.InternalError(c, "获取仓库列表失败")
		return
	}
	response.List(c, repos, total, page, pageSize)
}

func buildGitRepoListOptions(c *gin.Context, page, pageSize int) *repository.GitRepoListOptions {
	sortField := c.Query("sort_by")
	if sortField == "" {
		sortField = c.Query("sort_field")
	}

	opts := &repository.GitRepoListOptions{
		Page:      page,
		PageSize:  pageSize,
		Name:      GetStringFilter(c, "name"),
		URL:       GetStringFilter(c, "url"),
		Status:    c.Query("status"),
		AuthType:  c.Query("auth_type"),
		SortField: sortField,
		SortOrder: c.Query("sort_order"),
	}
	if syncEnabledStr := c.Query("sync_enabled"); syncEnabledStr != "" {
		syncEnabled := syncEnabledStr == "true"
		opts.SyncEnabled = &syncEnabled
	}
	parseGitRepoDateRange(c, opts)
	return opts
}

func parseGitRepoDateRange(c *gin.Context, opts *repository.GitRepoListOptions) {
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

// CreateRepo 创建仓库
func (h *GitRepoHandler) CreateRepo(c *gin.Context) {
	if !requireRepositoryValidatePermission(c) {
		return
	}

	var req CreateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	if err := validateGitCreateRequest(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	repo, err := h.svc.CreateRepo(c.Request.Context(), req.ToModel())
	if err != nil {
		response.BadRequestFromErr(c, err)
		return
	}
	response.Created(c, repo)
}

// GetRepo 获取仓库详情
func (h *GitRepoHandler) GetRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	repo, err := h.svc.GetRepo(c.Request.Context(), id)
	if err != nil {
		respondResourceError(c, "GIT", "获取仓库详情失败", "仓库不存在", repository.ErrGitRepositoryNotFound, resourceErrorModeInternal, err)
		return
	}
	response.Success(c, repo)
}

// UpdateRepo 更新仓库
func (h *GitRepoHandler) UpdateRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	var req UpdateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	current, err := h.svc.GetRepo(c.Request.Context(), id)
	if err != nil {
		respondResourceError(c, "GIT", "获取仓库详情失败", "仓库不存在", repository.ErrGitRepositoryNotFound, resourceErrorModeInternal, err)
		return
	}
	if err := validateGitUpdateRequest(current, &req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	repo, err := h.svc.UpdateRepo(c.Request.Context(), id, req.DefaultBranch, req.AuthType, req.AuthConfig, req.SyncEnabled, req.SyncInterval, req.MaxFailures)
	if err != nil {
		respondResourceError(c, "GIT", "更新仓库失败", "仓库不存在", repository.ErrGitRepositoryNotFound, resourceErrorModeInternal, err)
		return
	}
	response.Success(c, repo)
}

// DeleteRepo 删除仓库
func (h *GitRepoHandler) DeleteRepo(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.svc.DeleteRepo(c.Request.Context(), id); err != nil {
		response.InternalError(c, "删除失败")
		return
	}
	response.Message(c, "删除成功")
}

// ValidateRepo 验证仓库（创建前获取分支列表）
func (h *GitRepoHandler) ValidateRepo(c *gin.Context) {
	if !requireRepositoryValidatePermission(c) {
		return
	}

	var req ValidateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	if err := validateGitAuthType(req.AuthType); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.svc.ValidateRepo(c.Request.Context(), req.URL, req.AuthType, req.AuthConfig)
	if err != nil {
		response.BadRequestFromErr(c, err)
		return
	}
	response.Success(c, result)
}

// GetStats 获取 Git 仓库统计信息
func (h *GitRepoHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "GIT", "获取统计信息失败", err)
		return
	}
	response.Success(c, stats)
}
