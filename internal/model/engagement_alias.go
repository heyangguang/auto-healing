package model

import engagementmodel "github.com/company/auto-healing/internal/modules/engagement/model"

const (
	SiteMessageCategorySystemUpdate  = engagementmodel.SiteMessageCategorySystemUpdate
	SiteMessageCategoryFaultAlert    = engagementmodel.SiteMessageCategoryFaultAlert
	SiteMessageCategoryServiceNotice = engagementmodel.SiteMessageCategoryServiceNotice
	SiteMessageCategoryProductNews   = engagementmodel.SiteMessageCategoryProductNews
	SiteMessageCategoryActivity      = engagementmodel.SiteMessageCategoryActivity
	SiteMessageCategorySecurity      = engagementmodel.SiteMessageCategorySecurity
	SiteMessageCategoryAnnouncement  = engagementmodel.SiteMessageCategoryAnnouncement
)

var AllSiteMessageCategories = engagementmodel.AllSiteMessageCategories

type DashboardConfig = engagementmodel.DashboardConfig
type DashboardConfigData = engagementmodel.DashboardConfigData
type DashboardWorkspace = engagementmodel.DashboardWorkspace
type DashboardWidgetItem = engagementmodel.DashboardWidgetItem
type DashboardLayoutItem = engagementmodel.DashboardLayoutItem
type SystemWorkspace = engagementmodel.SystemWorkspace
type RoleWorkspace = engagementmodel.RoleWorkspace
type SystemWorkspaceConfig = engagementmodel.SystemWorkspaceConfig

type RetryConfig = engagementmodel.RetryConfig
type NotificationChannel = engagementmodel.NotificationChannel
type NotificationTemplate = engagementmodel.NotificationTemplate
type NotificationLog = engagementmodel.NotificationLog
type NotificationTriggerConfig = engagementmodel.NotificationTriggerConfig
type TaskNotificationConfig = engagementmodel.TaskNotificationConfig

type SiteMessageCategoryInfo = engagementmodel.SiteMessageCategoryInfo
type SiteMessage = engagementmodel.SiteMessage
type SiteMessageRead = engagementmodel.SiteMessageRead
type SiteMessageWithReadStatus = engagementmodel.SiteMessageWithReadStatus

type UserFavorite = engagementmodel.UserFavorite
type UserRecent = engagementmodel.UserRecent
type UserPreference = engagementmodel.UserPreference
