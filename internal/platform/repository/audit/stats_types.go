package audit

import "github.com/company/auto-healing/internal/model"

type AuditStats struct {
	TotalCount    int64        `json:"total_count"`
	SuccessCount  int64        `json:"success_count"`
	FailedCount   int64        `json:"failed_count"`
	HighRiskCount int64        `json:"high_risk_count"`
	ActionStats   []ActionStat `json:"action_stats"`
	TodayCount    int64        `json:"today_count"`
	WeekCount     int64        `json:"week_count"`
}

type PlatformAuditStats struct {
	TotalCount   int64        `json:"total_count"`
	LoginCount   int64        `json:"login_count"`
	OperCount    int64        `json:"operation_count"`
	SuccessCount int64        `json:"success_count"`
	FailedCount  int64        `json:"failed_count"`
	TodayCount   int64        `json:"today_count"`
	WeekCount    int64        `json:"week_count"`
	ActionStats  []ActionStat `json:"action_stats"`
}

type ActionStat struct {
	Action string `json:"action" gorm:"column:action"`
	Count  int64  `json:"count" gorm:"column:count"`
}

type UserRanking struct {
	UserID   string `json:"user_id" gorm:"column:user_id"`
	Username string `json:"username" gorm:"column:username"`
	Count    int64  `json:"count" gorm:"column:count"`
}

type ActionGroupItem struct {
	Action       string `json:"action" gorm:"column:action"`
	ResourceType string `json:"resource_type" gorm:"column:resource_type"`
	Username     string `json:"username" gorm:"column:username"`
	Count        int64  `json:"count" gorm:"column:count"`
}

type ResourceTypeGroupItem struct {
	ResourceType string `json:"resource_type" gorm:"column:resource_type"`
	Count        int64  `json:"count" gorm:"column:count"`
}

type TrendItem struct {
	Date  string `json:"date" gorm:"column:date"`
	Count int64  `json:"count" gorm:"column:count"`
}

type HighRiskLog struct {
	model.AuditLog
	RiskReason string `json:"risk_reason" gorm:"-"`
}
