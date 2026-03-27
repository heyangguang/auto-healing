package httpapi

import (
	"github.com/company/auto-healing/internal/middleware"
	"github.com/gin-gonic/gin"
)

func (r Registrar) RegisterCommonRoutes(common *gin.RouterGroup) {
	common.GET("/search", r.deps.Search.GlobalSearch)

	userPrefs := common.Group("/user/preferences")
	userPrefs.GET("", r.deps.Preference.GetPreferences)
	userPrefs.PUT("", r.deps.Preference.UpdatePreferences)
	userPrefs.PATCH("", r.deps.Preference.PatchPreferences)

	userFavorites := common.Group("/user/favorites")
	userFavorites.GET("", r.deps.Activity.ListFavorites)
	userFavorites.POST("", r.deps.Activity.AddFavorite)
	userFavorites.DELETE("/:menu_key", r.deps.Activity.RemoveFavorite)

	userRecents := common.Group("/user/recents")
	userRecents.GET("", r.deps.Activity.ListRecents)
	userRecents.POST("", r.deps.Activity.RecordRecent)

	workbench := common.Group("/workbench")
	workbench.GET("/overview", r.deps.Workbench.GetOverview)
	workbench.GET("/activities", middleware.RequirePermission("audit:list"), r.deps.Workbench.GetActivities)
	workbench.GET("/schedule-calendar", middleware.RequirePermission("task:list"), r.deps.Workbench.GetScheduleCalendar)
	workbench.GET("/announcements", r.deps.Workbench.GetAnnouncements)
	workbench.GET("/favorites", r.deps.Workbench.GetFavorites)

	common.GET("/site-messages/categories", r.deps.SiteMessage.GetCategories)
}

func (r Registrar) RegisterPlatformRoutes(platform *gin.RouterGroup) {
	siteMessages := platform.Group("/site-messages")
	siteMessages.POST("", middleware.RequirePermission("platform:messages:send"), r.deps.SiteMessage.CreateMessage)
	siteMessages.GET("/settings", middleware.RequirePermission("site-message:settings:view"), r.deps.SiteMessage.GetSettings)
	siteMessages.PUT("/settings", middleware.RequirePermission("site-message:settings:manage"), r.deps.SiteMessage.UpdateSettings)
}
