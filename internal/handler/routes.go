package handler

// ==================== 路由注册规范 ====================
//
// 【重要】固定路径必须注册在参数路由（/:id）之前！
//
// Gin 使用基数树匹配路由，如果 /:id 注册在固定路径之前，
// 固定路径（如 /stats）会被当作 :id 参数匹配，导致返回"无效的ID"。
//
// ✅ 正确顺序：
//   group.GET("/stats", handler)   // 固定路径在前
//   group.GET("/:id", handler)     // 参数路由在后
//
// ❌ 错误顺序：
//   group.GET("/:id", handler)     // 参数路由在前 → 吞掉 /stats
//   group.GET("/stats", handler)   // 永远匹配不到
//
// 同理适用于 /validate, /export, /pending 等所有固定子路径。
// =======================================================

import (
	"github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handlers 所有处理器集合
type Handlers struct {
	Auth               *AuthHandler
	User               *UserHandler
	TenantUser         *TenantUserHandler // 租户级用户管理
	Role               *RoleHandler
	Permission         *PermissionHandler
	Plugin             *PluginHandler
	CMDB               *CMDBHandler
	Secrets            *SecretsHandler
	GitRepo            *GitRepoHandler
	Playbook           *PlaybookHandler
	Execution          *ExecutionHandler
	Schedule           *ScheduleHandler
	Notification       *NotificationHandler
	Healing            *HealingHandler
	Dashboard          *DashboardHandler
	Preference         *PreferenceHandler
	Audit              *AuditHandler
	PlatformAudit      *PlatformAuditHandler
	Activity           *UserActivityHandler
	Search             *SearchHandler
	SiteMessage        *SiteMessageHandler
	PlatformSettings   *PlatformSettingsHandler
	Tenant             *TenantHandler
	Workbench          *WorkbenchHandler
	Dictionary         *DictionaryHandler
	Impersonation      *ImpersonationHandler
	CommandBlacklist   *CommandBlacklistHandler
	BlacklistExemption *BlacklistExemptionHandler
}

// NewHandlers 创建所有处理器
func NewHandlers(cfg *config.Config) *Handlers {
	authHandler := NewAuthHandler(cfg)
	handlers := &Handlers{
		Auth:               authHandler,
		User:               NewUserHandler(authHandler.authSvc),
		TenantUser:         NewTenantUserHandler(authHandler.authSvc),
		Role:               NewRoleHandler(),
		Permission:         NewPermissionHandler(),
		Plugin:             NewPluginHandler(),
		CMDB:               NewCMDBHandler(),
		Secrets:            NewSecretsHandler(),
		GitRepo:            NewGitRepoHandler(),
		Playbook:           NewPlaybookHandler(),
		Execution:          NewExecutionHandler(),
		Schedule:           NewScheduleHandler(),
		Notification:       NewNotificationHandler(),
		Healing:            NewHealingHandler(),
		Dashboard:          NewDashboardHandler(),
		Preference:         NewPreferenceHandler(),
		Audit:              NewAuditHandler(),
		PlatformAudit:      NewPlatformAuditHandler(),
		Activity:           NewUserActivityHandler(),
		Search:             NewSearchHandler(),
		SiteMessage:        NewSiteMessageHandler(),
		PlatformSettings:   NewPlatformSettingsHandler(),
		Tenant:             NewTenantHandler(authHandler.authSvc),
		Workbench:          NewWorkbenchHandler(),
		Dictionary:         NewDictionaryHandler(),
		Impersonation:      NewImpersonationHandler(),
		CommandBlacklist:   NewCommandBlacklistHandler(),
		BlacklistExemption: NewBlacklistExemptionHandler(),
	}
	registerHandlerCleanup(handlers.Execution.Shutdown)
	registerHandlerCleanup(handlers.Healing.Shutdown)
	registerHandlerCleanup(handlers.Plugin.Shutdown)
	registerHandlerCleanup(handlers.GitRepo.Shutdown)
	return handlers
}

// SetupRoutes 设置所有路由
// 路由分组：
//   - public:   公开路由，无需任何认证
//   - auth:     认证相关（登录公开 + 登录后个人信息）
//   - common:   公共路由，需认证，无需租户上下文（平台+租户通用）
//   - platform: 平台级路由，需认证 + 平台管理员权限
//   - tenant:   租户级路由，需认证 + 租户上下文
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	h := NewHandlers(cfg)
	api := r.Group("/api/v1")

	// ==================== 1. 公开路由（Public） ====================
	setupPublicRoutes(api, h)

	// ==================== 2. 认证路由（Auth） ====================
	setupAuthRoutes(api, h)

	// ==================== 3. 公共路由（Common） ====================
	setupCommonRoutes(api, h)

	// ==================== 4. 平台级路由（Platform） ====================
	setupPlatformRoutes(api, h)

	// ==================== 5. 租户级路由（Tenant） ====================
	setupTenantRoutes(api, h)
}

// ==================== 公开路由（Public） ====================
// 不需要任何认证，用于健康检查等公开接口
func setupPublicRoutes(api *gin.RouterGroup, h *Handlers) {
	public := api.Group("/public")
	{
		// 预留：健康检查、版本信息等公开接口
		_ = public
	}
}

// ==================== 认证路由（Auth） ====================
// 登录相关：公开子组（login/register）+ 认证子组（me/profile）
func setupAuthRoutes(api *gin.RouterGroup, h *Handlers) {
	auth := api.Group("/auth")

	// --- 公开（无需认证）---
	{
		auth.POST("/login", h.Auth.Login)
		auth.POST("/refresh", h.Auth.RefreshToken)
		auth.GET("/invitation/:token", ValidateInvitation)
		auth.POST("/register", RegisterByInvitation(h.Auth.GetAuthService()))
	}

	// --- 需要认证 ---
	authProtected := auth.Group("")
	authProtected.Use(middleware.JWTAuth(h.Auth.GetJWTService()))
	authProtected.Use(middleware.AuditMiddleware())
	{
		// /auth/me 需要返回当前“有效视角”下的用户信息：
		// - 普通租户用户：当前租户权限
		// - 平台管理员提权中：impersonation_accessor 权限
		authProtected.GET("/me",
			middleware.ImpersonationMiddleware(),
			middleware.CommonTenantMiddleware(),
			h.Auth.GetCurrentUser,
		)
		authProtected.GET("/profile", h.Auth.GetProfile)
		authProtected.PUT("/profile", h.Auth.UpdateProfile)
		authProtected.GET("/profile/login-history", h.Auth.GetLoginHistory)
		authProtected.GET("/profile/activities", h.Auth.GetProfileActivities)
		authProtected.PUT("/password", h.Auth.ChangePassword)
		authProtected.POST("/logout", h.Auth.Logout)
	}
}

// ==================== 公共路由（Common） ====================
// 需要认证，无需租户上下文，平台用户和租户用户均可访问
func setupCommonRoutes(api *gin.RouterGroup, h *Handlers) {
	common := api.Group("/common")
	common.Use(middleware.JWTAuth(h.Auth.GetJWTService()))
	common.Use(middleware.ImpersonationMiddleware())
	common.Use(middleware.CommonTenantMiddleware())
	common.Use(middleware.AuditMiddleware())
	{
		// -------------------- 全局搜索 --------------------
		common.GET("/search", h.Search.GlobalSearch)

		// -------------------- 用户偏好设置 --------------------
		userPrefs := common.Group("/user/preferences")
		{
			userPrefs.GET("", h.Preference.GetPreferences)
			userPrefs.PUT("", h.Preference.UpdatePreferences)
			userPrefs.PATCH("", h.Preference.PatchPreferences)
		}

		// -------------------- 用户收藏 --------------------
		userFavorites := common.Group("/user/favorites")
		{
			userFavorites.GET("", h.Activity.ListFavorites)
			userFavorites.POST("", h.Activity.AddFavorite)
			userFavorites.DELETE("/:menu_key", h.Activity.RemoveFavorite)
		}

		// -------------------- 最近访问 --------------------
		userRecents := common.Group("/user/recents")
		{
			userRecents.GET("", h.Activity.ListRecents)
			userRecents.POST("", h.Activity.RecordRecent)
		}

		// -------------------- 用户租户列表 --------------------
		common.GET("/user/tenants", h.Tenant.GetUserTenants)

		// -------------------- Workbench (工作台) --------------------
		workbench := common.Group("/workbench")
		{
			workbench.GET("/overview", h.Workbench.GetOverview)
			workbench.GET("/activities", middleware.RequirePermission("audit:list"), h.Workbench.GetActivities)
			workbench.GET("/schedule-calendar", middleware.RequirePermission("task:list"), h.Workbench.GetScheduleCalendar)
			workbench.GET("/announcements", h.Workbench.GetAnnouncements)
			workbench.GET("/favorites", h.Workbench.GetFavorites)
		}

		// -------------------- 站内信分类（静态枚举） --------------------
		common.GET("/site-messages/categories", h.SiteMessage.GetCategories)

		// -------------------- 字典值查询（只读） --------------------
		dictionaries := common.Group("/dictionaries")
		{
			dictionaries.GET("", h.Dictionary.ListDictionaries)
			dictionaries.GET("/types", h.Dictionary.ListTypes)
		}
	}
}
