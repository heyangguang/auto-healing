package httpapi

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/company/auto-healing/internal/middleware"
	accessrepo "github.com/company/auto-healing/internal/modules/access/repository"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WorkbenchHandler 工作台处理器
type WorkbenchHandler struct {
	repo         *engagementrepo.WorkbenchRepository
	activityRepo *engagementrepo.UserActivityRepository
	userRepo     *accessrepo.UserRepository
}

type WorkbenchHandlerDeps struct {
	WorkbenchRepo *engagementrepo.WorkbenchRepository
	ActivityRepo  *engagementrepo.UserActivityRepository
	UserRepo      *accessrepo.UserRepository
}

type workbenchSection struct {
	key string
	fn  func() (interface{}, error)
}

func NewWorkbenchHandlerWithDeps(deps WorkbenchHandlerDeps) *WorkbenchHandler {
	return &WorkbenchHandler{
		repo:         deps.WorkbenchRepo,
		activityRepo: deps.ActivityRepo,
		userRepo:     deps.UserRepo,
	}
}

// GetOverview 获取工作台综合概览（按权限动态返回 section）
// GET /api/v1/workbench/overview
func (h *WorkbenchHandler) GetOverview(c *gin.Context) {
	if !requireTenantContext(c, "当前工作台接口需要租户上下文，请先选择租户或通过 Impersonation 进入租户") {
		return
	}
	ctx := c.Request.Context()
	permissions := middleware.GetPermissions(c)
	result, lastErr := h.loadWorkbenchOverviewSections(ctx, permissions)
	if lastErr != nil {
		response.InternalError(c, "获取工作台概览失败: "+lastErr.Error())
		return
	}
	typed, err := newWorkbenchOverviewResponse(result)
	if err != nil {
		response.InternalError(c, "获取工作台概览失败: "+err.Error())
		return
	}
	response.Success(c, typed)
}

func (h *WorkbenchHandler) loadWorkbenchOverviewSections(ctx context.Context, permissions []string) (map[string]interface{}, error) {
	sections := h.workbenchOverviewSections(ctx, permissions)
	state := newConcurrentSectionState(len(sections))
	var wg sync.WaitGroup
	for _, sec := range sections {
		wg.Add(1)
		go h.runWorkbenchSection(sec, &wg, state)
	}
	wg.Wait()
	return state.resultAndError()
}

func (h *WorkbenchHandler) workbenchOverviewSections(ctx context.Context, permissions []string) []workbenchSection {
	sections := []workbenchSection{
		{key: "system_health", fn: func() (interface{}, error) { return h.repo.GetSystemHealth(ctx) }},
		{key: "resource_overview", fn: func() (interface{}, error) { return h.repo.GetResourceOverview(ctx, permissions) }},
	}
	if workbenchHasPermission(permissions, "healing:instances:view") {
		sections = append(sections, workbenchSection{key: "healing_stats", fn: func() (interface{}, error) { return h.repo.GetHealingStats(ctx) }})
	}
	if workbenchHasPermission(permissions, "plugin:list") {
		sections = append(sections,
			workbenchSection{key: "incident_stats", fn: func() (interface{}, error) { return h.repo.GetIncidentStats(ctx) }},
			workbenchSection{key: "host_stats", fn: func() (interface{}, error) { return h.repo.GetHostStats(ctx) }},
		)
	}
	return sections
}

func (h *WorkbenchHandler) runWorkbenchSection(sec workbenchSection, wg *sync.WaitGroup, state *concurrentSectionState) {
	defer wg.Done()
	data, err := safeSectionLoad(sec.fn)
	if err != nil {
		state.addError(sec.key, err)
		return
	}
	state.addResult(sec.key, data)
}

// GetActivities 获取活动动态
// GET /api/v1/workbench/activities?limit=10
func (h *WorkbenchHandler) GetActivities(c *gin.Context) {
	if !requireTenantContext(c, "当前工作台接口需要租户上下文，请先选择租户或通过 Impersonation 进入租户") {
		return
	}
	limit := 10
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	items, err := h.repo.GetRecentActivities(c.Request.Context(), limit)
	if err != nil {
		respondInternalError(c, "WORKBENCH", "获取活动动态失败", err)
		return
	}

	response.Success(c, items)
}

// GetScheduleCalendar 获取定时任务日历
// GET /api/v1/workbench/schedule-calendar?year=2026&month=2
func (h *WorkbenchHandler) GetScheduleCalendar(c *gin.Context) {
	if !requireTenantContext(c, "当前工作台接口需要租户上下文，请先选择租户或通过 Impersonation 进入租户") {
		return
	}
	yearStr := c.Query("year")
	monthStr := c.Query("month")

	if yearStr == "" || monthStr == "" {
		response.BadRequest(c, "year 和 month 参数必填")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		response.BadRequest(c, "无效的 year 参数")
		return
	}

	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		response.BadRequest(c, "无效的 month 参数（1-12）")
		return
	}

	dates, err := h.repo.GetScheduleCalendar(c.Request.Context(), year, month)
	if err != nil {
		respondInternalError(c, "WORKBENCH", "获取定时任务日历失败", err)
		return
	}

	response.Success(c, dates)
}

// GetAnnouncements 获取系统公告
// GET /api/v1/workbench/announcements?limit=5
func (h *WorkbenchHandler) GetAnnouncements(c *gin.Context) {
	if !requireTenantContext(c, "当前工作台接口需要租户上下文，请先选择租户或通过 Impersonation 进入租户") {
		return
	}
	limit := 5
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	// 获取用户创建时间，不显示注册前的公告
	var userCreatedAt time.Time
	if userID, err := uuid.Parse(middleware.GetUserID(c)); err == nil {
		user, err := h.userRepo.GetByID(c.Request.Context(), userID)
		if err != nil && !errors.Is(err, accessrepo.ErrUserNotFound) {
			respondInternalError(c, "WORKBENCH", "获取系统公告失败", err)
			return
		}
		if err == nil {
			userCreatedAt = user.CreatedAt
		}
	}

	items, err := h.repo.GetAnnouncements(c.Request.Context(), limit, userCreatedAt)
	if err != nil {
		respondInternalError(c, "WORKBENCH", "获取系统公告失败", err)
		return
	}

	response.Success(c, items)
}

// GetFavorites 获取用户收藏（复用 user_favorites 表，与侧边栏一致）
// GET /api/v1/workbench/favorites
func (h *WorkbenchHandler) GetFavorites(c *gin.Context) {
	if !requireTenantContext(c, "当前工作台接口需要租户上下文，请先选择租户或通过 Impersonation 进入租户") {
		return
	}
	userID, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		response.BadRequest(c, "无效的用户ID")
		return
	}

	favorites, err := h.activityRepo.ListFavorites(c.Request.Context(), userID)
	if err != nil {
		respondInternalError(c, "WORKBENCH", "获取收藏失败", err)
		return
	}

	// 转换为工作台前端所需格式
	items := make([]map[string]string, 0, len(favorites))
	for _, f := range favorites {
		items = append(items, map[string]string{
			"key":   f.MenuKey,
			"label": f.Name,
			"path":  f.Path,
		})
	}

	response.Success(c, items)
}

// ==================== 权限辅助函数 ====================

// workbenchHasPermission 检查用户是否有指定权限（含通配符匹配）
func workbenchHasPermission(userPermissions []string, required string) bool {
	for _, p := range userPermissions {
		// 超级管理员通配符
		if p == "*" {
			return true
		}
		// 精确匹配
		if p == required {
			return true
		}
		// 模块级通配符 (e.g., "plugin:*" 匹配 "plugin:list")
		if strings.HasSuffix(p, ":*") {
			module := strings.TrimSuffix(p, ":*")
			if strings.HasPrefix(required, module+":") {
				return true
			}
		}
	}
	return false
}
