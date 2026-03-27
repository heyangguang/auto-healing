package healing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	engagementrepo "github.com/company/auto-healing/internal/modules/engagement/repository"
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/query"
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"github.com/google/uuid"
)

// NodeExecutors 节点执行器集合
type NodeExecutors struct {
	cmdbRepo         *cmdbrepo.CMDBItemRepository
	notificationRepo *engagementrepo.NotificationRepository
	notificationSvc  *notification.Service
}

// NewNodeExecutors 创建节点执行器
func NewNodeExecutors() *NodeExecutors {
	return NewNodeExecutorsWithDependencies(
		cmdbrepo.NewCMDBItemRepository(),
		engagementrepo.NewNotificationRepository(database.DB),
		notification.NewConfiguredService(database.DB),
	)
}

func NewNodeExecutorsWithDependencies(cmdbRepo *cmdbrepo.CMDBItemRepository, notificationRepo *engagementrepo.NotificationRepository, notificationSvc *notification.Service) *NodeExecutors {
	return &NodeExecutors{
		cmdbRepo:         cmdbRepo,
		notificationRepo: notificationRepo,
		notificationSvc:  notificationSvc,
	}
}

// HostExtractorConfig 主机提取配置
type HostExtractorConfig struct {
	Source  string `json:"source"`  // 提取来源: title, description, affected_ci, raw_data.xxx
	Pattern string `json:"pattern"` // 正则表达式模式
	Field   string `json:"field"`   // 如果来源是 raw_data，指定具体字段
}

// ExecuteHostExtractor 执行主机提取
func (e *NodeExecutors) ExecuteHostExtractor(_ context.Context, instance *model.FlowInstance, config map[string]interface{}) ([]string, error) {
	cfg := parseHostExtractorConfig(config)
	incident := e.getIncidentFromContext(instance)
	if incident == nil {
		return nil, nil
	}
	return extractHostsFromIncident(incident, cfg)
}

func parseHostExtractorConfig(config map[string]interface{}) *HostExtractorConfig {
	cfg := &HostExtractorConfig{
		Source:  "description",
		Pattern: `\b(?:\d{1,3}\.){3}\d{1,3}\b|[a-zA-Z][a-zA-Z0-9-]*(?:\.[a-zA-Z0-9-]+)+`,
	}
	if source, ok := config["source"].(string); ok {
		cfg.Source = source
	}
	if pattern, ok := config["pattern"].(string); ok {
		cfg.Pattern = pattern
	}
	if field, ok := config["field"].(string); ok {
		cfg.Field = field
	}
	return cfg
}

func extractHostsFromIncident(incident *model.Incident, cfg *HostExtractorConfig) ([]string, error) {
	re, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, err
	}
	return uniqueHosts(re.FindAllString(hostExtractorSourceText(incident, cfg), -1)), nil
}

func hostExtractorSourceText(incident *model.Incident, cfg *HostExtractorConfig) string {
	switch cfg.Source {
	case "title":
		return incident.Title
	case "description":
		return incident.Description
	case "affected_ci":
		return incident.AffectedCI
	case "affected_service":
		return incident.AffectedService
	case "raw_data":
		if incident.RawData != nil && cfg.Field != "" {
			if val, ok := incident.RawData[cfg.Field]; ok {
				return toString(val)
			}
		}
	}
	return incident.Description
}

func uniqueHosts(matches []string) []string {
	hostSet := make(map[string]bool)
	var hosts []string
	for _, match := range matches {
		host := strings.TrimSpace(match)
		if host == "" || hostSet[host] {
			continue
		}
		hostSet[host] = true
		hosts = append(hosts, host)
	}
	return hosts
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

// NotificationConfig 通知配置
type NotificationConfig struct {
	ChannelID  int64             `json:"channel_id"`
	TemplateID int64             `json:"template_id"`
	Recipients []string          `json:"recipients"`
	Variables  map[string]string `json:"variables"`
}

// ExecuteNotification 执行通知
func (e *NodeExecutors) ExecuteNotification(ctx context.Context, instance *model.FlowInstance, config map[string]interface{}) error {
	sendReq := e.buildNotificationSendRequest(instance, config)
	notifSvc := e.newNotificationService()
	_, err := notifSvc.Send(ctx, sendReq)
	return err
}

func (e *NodeExecutors) newNotificationService() *notification.Service {
	return e.notificationSvc
}

func (e *NodeExecutors) buildNotificationSendRequest(instance *model.FlowInstance, config map[string]interface{}) notification.SendNotificationRequest {
	sendReq := notification.SendNotificationRequest{
		Variables:  e.buildNotificationVariables(instance, config),
		ChannelIDs: parseNotificationChannelIDs(config),
		TemplateID: parseNotificationTemplateID(config),
	}
	if subject, ok := config["subject"].(string); ok {
		sendReq.Subject = subject
	}
	if body, ok := config["body"].(string); ok {
		sendReq.Body = body
	}
	return sendReq
}

func (e *NodeExecutors) buildNotificationVariables(instance *model.FlowInstance, config map[string]interface{}) map[string]interface{} {
	variables := make(map[string]interface{})
	incident := e.getIncidentFromContext(instance)
	if incident != nil {
		variables["incident_id"] = incident.ID.String()
		variables["incident_title"] = incident.Title
		variables["incident_severity"] = incident.Severity
		variables["incident_status"] = incident.Status
	}
	if configVars, ok := config["variables"].(map[string]interface{}); ok {
		for key, value := range configVars {
			variables[key] = toString(value)
		}
	}
	return variables
}

func parseNotificationChannelIDs(config map[string]interface{}) []uuid.UUID {
	var channelIDs []uuid.UUID
	if idStrs, ok := config["channel_ids"].([]interface{}); ok {
		for _, raw := range idStrs {
			if id, err := uuid.Parse(fmt.Sprint(raw)); err == nil {
				channelIDs = append(channelIDs, id)
			}
		}
		return channelIDs
	}
	if idStr, ok := config["channel_id"].(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			channelIDs = append(channelIDs, id)
		}
	}
	return channelIDs
}

func parseNotificationTemplateID(config map[string]interface{}) *uuid.UUID {
	tmplStr, ok := config["template_id"].(string)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(tmplStr)
	if err != nil {
		return nil
	}
	return &id
}

// getIncidentFromContext 从实例上下文获取工单
func (e *NodeExecutors) getIncidentFromContext(instance *model.FlowInstance) *model.Incident {
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

	var incident model.Incident
	if err := json.Unmarshal(data, &incident); err != nil {
		return nil
	}

	return &incident
}
