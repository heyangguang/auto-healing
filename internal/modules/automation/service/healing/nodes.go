package healing

import (
	"context"
	"encoding/json"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/modules/automation/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/query"
	platformmodel "github.com/company/auto-healing/internal/platform/model"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

// NodeExecutors 节点执行器集合
type NodeExecutors struct {
	cmdbRepo         *cmdbrepo.CMDBItemRepository
	notificationRepo *engagementrepo.NotificationRepository
	notificationSvc  *notification.Service
}

type NodeExecutorsDeps struct {
	CMDBRepo         *cmdbrepo.CMDBItemRepository
	NotificationRepo *engagementrepo.NotificationRepository
	NotificationSvc  *notification.Service
}

// NewNodeExecutors 创建节点执行器
func NewNodeExecutors() *NodeExecutors {
	return NewNodeExecutorsWithDB(database.DB)
}

func NewNodeExecutorsWithDB(db *gorm.DB) *NodeExecutors {
	return NewNodeExecutorsWithDeps(NodeExecutorsDeps{
		CMDBRepo:         cmdbrepo.NewCMDBItemRepositoryWithDB(db),
		NotificationRepo: engagementrepo.NewNotificationRepository(db),
		NotificationSvc:  notification.NewConfiguredService(db),
	})
}

func NewNodeExecutorsWithDependencies(cmdbRepo *cmdbrepo.CMDBItemRepository, notificationRepo *engagementrepo.NotificationRepository, notificationSvc *notification.Service) *NodeExecutors {
	return NewNodeExecutorsWithDeps(NodeExecutorsDeps{
		CMDBRepo:         cmdbRepo,
		NotificationRepo: notificationRepo,
		NotificationSvc:  notificationSvc,
	})
}

func NewNodeExecutorsWithDeps(deps NodeExecutorsDeps) *NodeExecutors {
	return &NodeExecutors{
		cmdbRepo:         deps.CMDBRepo,
		notificationRepo: deps.NotificationRepo,
		notificationSvc:  deps.NotificationSvc,
	}
}

// CMDBValidatorConfig CMDB 校验配置
type CMDBValidatorConfig struct {
	RequireActive bool   `json:"require_active"` // 是否要求资产状态为活跃
	RequireType   string `json:"require_type"`   // 要求的资产类型
	FailOnEmpty   bool   `json:"fail_on_empty"`  // 无有效主机时是否失败
}

// ExecuteCMDBValidator 执行 CMDB 校验
func (e *NodeExecutors) ExecuteCMDBValidator(ctx context.Context, instance *model.FlowInstance, config map[string]interface{}, hosts []string) ([]string, error) {
	// 解析配置
	cfg := &CMDBValidatorConfig{
		RequireActive: true,
		FailOnEmpty:   true,
	}

	if requireActive, ok := config["require_active"].(bool); ok {
		cfg.RequireActive = requireActive
	}
	if requireType, ok := config["require_type"].(string); ok {
		cfg.RequireType = requireType
	}
	if failOnEmpty, ok := config["fail_on_empty"].(bool); ok {
		cfg.FailOnEmpty = failOnEmpty
	}

	// 验证每个主机
	var validHosts []string
	for _, host := range hosts {
		// 查询 CMDB (使用 hostname 作为搜索条件)
		items, _, err := e.cmdbRepo.List(ctx, 1, 10, nil, "", "", "", host, query.StringFilter{}, nil, "", "")
		if err != nil {
			continue
		}

		for _, item := range items {
			// 检查状态（只有 active 才允许）
			if cfg.RequireActive && item.Status != "active" {
				continue
			}
			// 检查类型
			if cfg.RequireType != "" && item.Type != cfg.RequireType {
				continue
			}
			// 匹配成功
			validHosts = append(validHosts, host)
			break
		}
	}

	// 检查是否有有效主机
	if len(validHosts) == 0 && cfg.FailOnEmpty {
		return nil, nil // 返回空表示失败
	}

	return validHosts, nil
}

// getIncidentFromContext 从实例上下文获取工单
func (e *NodeExecutors) getIncidentFromContext(instance *model.FlowInstance) *platformmodel.Incident {
	if instance.Context == nil {
		return nil
	}

	incidentData, ok := instance.Context["incident"]
	if !ok {
		return nil
	}

	// 转换为 Incident
	data, err := json.Marshal(incidentData)
	if err != nil {
		return nil
	}

	var incident platformmodel.Incident
	if err := json.Unmarshal(data, &incident); err != nil {
		return nil
	}

	return &incident
}
