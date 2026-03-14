package healing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	appconf "github.com/company/auto-healing/internal/config"
	"github.com/company/auto-healing/internal/database"
	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/notification"
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/company/auto-healing/internal/repository"
	"github.com/google/uuid"
)

// NodeExecutors 节点执行器集合
type NodeExecutors struct {
	cmdbRepo         *repository.CMDBItemRepository
	notificationRepo *repository.NotificationRepository
}

// NewNodeExecutors 创建节点执行器
func NewNodeExecutors() *NodeExecutors {
	return &NodeExecutors{
		cmdbRepo:         repository.NewCMDBItemRepository(),
		notificationRepo: repository.NewNotificationRepository(database.DB),
	}
}

// HostExtractorConfig 主机提取配置
type HostExtractorConfig struct {
	Source  string `json:"source"`  // 提取来源: title, description, affected_ci, raw_data.xxx
	Pattern string `json:"pattern"` // 正则表达式模式
	Field   string `json:"field"`   // 如果来源是 raw_data，指定具体字段
}

// ExecuteHostExtractor 执行主机提取
func (e *NodeExecutors) ExecuteHostExtractor(ctx context.Context, instance *model.FlowInstance, config map[string]interface{}) ([]string, error) {
	// 解析配置
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

	// 获取工单信息
	incident := e.getIncidentFromContext(instance)
	if incident == nil {
		return nil, nil
	}

	// 获取提取来源文本
	var sourceText string
	switch cfg.Source {
	case "title":
		sourceText = incident.Title
	case "description":
		sourceText = incident.Description
	case "affected_ci":
		sourceText = incident.AffectedCI
	case "affected_service":
		sourceText = incident.AffectedService
	case "raw_data":
		if incident.RawData != nil && cfg.Field != "" {
			if val, ok := incident.RawData[cfg.Field]; ok {
				sourceText = toString(val)
			}
		}
	default:
		sourceText = incident.Description
	}

	// 使用正则提取主机
	re, err := regexp.Compile(cfg.Pattern)
	if err != nil {
		return nil, err
	}

	matches := re.FindAllString(sourceText, -1)

	// 去重
	hostSet := make(map[string]bool)
	var hosts []string
	for _, match := range matches {
		host := strings.TrimSpace(match)
		if host != "" && !hostSet[host] {
			hostSet[host] = true
			hosts = append(hosts, host)
		}
	}

	return hosts, nil
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
	// 解析配置（使用 JSON 序列化避免逐字段赋值）
	configData, _ := json.Marshal(config)
	cfg := &NotificationConfig{}
	_ = json.Unmarshal(configData, cfg)

	// 构建变量
	variables := make(map[string]string)

	// 从实例上下文中获取变量
	incident := e.getIncidentFromContext(instance)
	if incident != nil {
		variables["incident_id"] = incident.ID.String()
		variables["incident_title"] = incident.Title
		variables["incident_severity"] = incident.Severity
		variables["incident_status"] = incident.Status
	}

	// 合并配置中的变量
	if configVars, ok := config["variables"].(map[string]interface{}); ok {
		for k, v := range configVars {
			variables[k] = toString(v)
		}
	}

	// 调用通知服务发送通知
	cp := appconf.GetAppConfig()
	notifSvc := notification.NewService(
		database.DB,
		cp.Name,
		cp.URL,
		cp.Version,
	)

	sendReq := notification.SendNotificationRequest{
		Variables: make(map[string]interface{}),
	}
	for k, v := range variables {
		sendReq.Variables[k] = v
	}

	// 解析 channel_ids（支持 UUID 数组或单个 channel_id）
	var channelIDs []uuid.UUID
	if idStrs, ok := config["channel_ids"].([]interface{}); ok {
		for _, s := range idStrs {
			if id, err := uuid.Parse(fmt.Sprint(s)); err == nil {
				channelIDs = append(channelIDs, id)
			}
		}
	} else if idStr, ok := config["channel_id"].(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			channelIDs = append(channelIDs, id)
		}
	}
	sendReq.ChannelIDs = channelIDs

	// 解析 template_id
	if tmplStr, ok := config["template_id"].(string); ok {
		if id, err := uuid.Parse(tmplStr); err == nil {
			sendReq.TemplateID = &id
		}
	}

	// 直接指定 subject/body（无模板时使用）
	if s, ok := config["subject"].(string); ok {
		sendReq.Subject = s
	}
	if b, ok := config["body"].(string); ok {
		sendReq.Body = b
	}

	_, err := notifSvc.Send(ctx, sendReq)
	return err

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
