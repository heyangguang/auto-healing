package httpapi

import (
	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	secretsSvc "github.com/company/auto-healing/internal/modules/secrets/service/secrets"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// CMDBHandler CMDB 处理器
type CMDBHandler struct {
	cmdbSvc   *plugin.CMDBService
	secretSvc *secretsSvc.Service
}

type CMDBHandlerDeps struct {
	Service       *plugin.CMDBService
	SecretService *secretsSvc.Service
}

// NewCMDBHandler 创建 CMDB 处理器
func NewCMDBHandler() *CMDBHandler {
	return NewCMDBHandlerWithDeps(CMDBHandlerDeps{
		Service: plugin.NewCMDBService(),
	})
}

func NewCMDBHandlerWithDeps(deps CMDBHandlerDeps) *CMDBHandler {
	return &CMDBHandler{
		cmdbSvc:   deps.Service,
		secretSvc: deps.SecretService,
	}
}

// ==================== Search Schema 声明 ====================

var cmdbSearchSchema = []SearchableField{
	{Key: "name", Label: "名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "资产名称", Column: "name"},
	{Key: "hostname", Label: "主机名", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "主机名", Column: "hostname"},
	{Key: "ip_address", Label: "IP 地址", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "IP 地址", Column: "ip_address"},
	{Key: "source_plugin_name", Label: "来源插件", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "插件名称", Column: "source_plugin_name"},
	{Key: "type", Label: "类型", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "服务器", Value: "server"}, {Label: "虚拟机", Value: "vm"},
		{Label: "网络设备", Value: "network"}, {Label: "容器", Value: "container"},
	}},
	{Key: "status", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "在线", Value: "online"}, {Label: "离线", Value: "offline"},
		{Label: "维护中", Value: "maintenance"},
	}},
	{Key: "environment", Label: "环境", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{
		{Label: "生产", Value: "production"}, {Label: "预发布", Value: "staging"},
		{Label: "测试", Value: "testing"}, {Label: "开发", Value: "development"},
	}},
	{Key: "has_plugin", Label: "关联插件", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
}

// GetCMDBSearchSchema 返回 CMDB 搜索 schema
func (h *CMDBHandler) GetCMDBSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": cmdbSearchSchema})
}
