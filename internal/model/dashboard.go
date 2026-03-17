package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DashboardConfig 用户 Dashboard 配置
type DashboardConfig struct {
	ID        uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;uniqueIndex:idx_dashboard_tenant_user"`
	TenantID  *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;uniqueIndex:idx_dashboard_tenant_user"`
	Config    JSON       `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	User *User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

// TableName 表名
func (DashboardConfig) TableName() string {
	return "dashboard_configs"
}

// DashboardConfigData Dashboard 配置数据（JSON 存储的内容）
type DashboardConfigData struct {
	ActiveWorkspaceID string               `json:"activeWorkspaceId"`
	Workspaces        []DashboardWorkspace `json:"workspaces"`
}

// DashboardWorkspace 工作区
type DashboardWorkspace struct {
	ID      string                `json:"id"`
	Name    string                `json:"name"`
	Widgets []DashboardWidgetItem `json:"widgets"`
	Layouts []DashboardLayoutItem `json:"layouts"`
}

// DashboardWidgetItem 工作区中的 Widget 实例
type DashboardWidgetItem struct {
	InstanceID string `json:"instanceId"`
	WidgetID   string `json:"widgetId"`
}

// DashboardLayoutItem 布局项
type DashboardLayoutItem struct {
	I    string `json:"i"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	MinW *int   `json:"minW,omitempty"`
	MinH *int   `json:"minH,omitempty"`
	MaxW *int   `json:"maxW,omitempty"`
	MaxH *int   `json:"maxH,omitempty"`
}

// SystemWorkspace 系统工作区（管理员创建的模板，可分配给角色）
type SystemWorkspace struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	Name        string     `json:"name" gorm:"type:varchar(200);not null"`
	Description string     `json:"description,omitempty" gorm:"type:text;default:''"`
	Config      JSON       `json:"config" gorm:"type:jsonb;not null;default:'{}'"`
	IsDefault   bool       `json:"is_default" gorm:"not null;default:false"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" gorm:"type:uuid"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
	UpdatedAt   time.Time  `json:"updated_at" gorm:"default:now()"`

	// 关联
	Creator *User  `json:"creator,omitempty" gorm:"foreignKey:CreatedBy"`
	Roles   []Role `json:"roles,omitempty" gorm:"many2many:role_workspaces;joinForeignKey:workspace_id;joinReferences:role_id"`
}

// TableName 表名
func (SystemWorkspace) TableName() string {
	return "system_workspaces"
}

// RoleWorkspace 角色-工作区关联
type RoleWorkspace struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    *uuid.UUID `json:"tenant_id,omitempty" gorm:"type:uuid;index"`
	RoleID      uuid.UUID  `json:"role_id" gorm:"type:uuid;not null"`
	WorkspaceID uuid.UUID  `json:"workspace_id" gorm:"type:uuid;not null"`
	CreatedAt   time.Time  `json:"created_at" gorm:"default:now()"`
}

// TableName 表名
func (RoleWorkspace) TableName() string {
	return "role_workspaces"
}

// SystemWorkspaceConfig 系统工作区配置（JSON 存储的内容）
type SystemWorkspaceConfig struct {
	Widgets []DashboardWidgetItem `json:"widgets"`
	Layouts []DashboardLayoutItem `json:"layouts"`
}

// Scan 实现 sql.Scanner 接口
func (d *DashboardConfigData) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal DashboardConfigData: %v", value)
	}
	return json.Unmarshal(bytes, d)
}

// Value 实现 driver.Valuer 接口
func (d DashboardConfigData) Value() (driver.Value, error) {
	return json.Marshal(d)
}
