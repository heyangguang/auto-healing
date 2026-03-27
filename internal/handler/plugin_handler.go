package handler

import (
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/service/plugin"
	"github.com/gin-gonic/gin"
)

// ══════ 插件搜索白名单 ══════
var pluginSearchSchema = []SearchableField{
	{Key: "name", Label: "名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "插件名称", Column: "name"},
	{Key: "description", Label: "描述", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "插件描述", Column: "description"},
	{Key: "type", Label: "类型", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{"ITSM", "itsm"}, {"CMDB", "cmdb"}}},
	{Key: "status", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{"活跃", "active"}, {"停用", "inactive"}, {"异常", "error"}}},
}

// ══════ 工单搜索白名单 ══════
var incidentSearchSchema = []SearchableField{
	{Key: "title", Label: "标题", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "事件标题", Column: "title"},
	{Key: "source_plugin_name", Label: "来源插件", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Column: "source_plugin_name", Placeholder: "插件名称"},
	{Key: "external_id", Label: "外部ID", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "exact", Column: "external_id", Placeholder: "外部工单ID"},
	{Key: "status", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{"待处理", "open"}, {"已关闭", "closed"}}},
	{Key: "healing_status", Label: "自愈状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{"待处理", "pending"}, {"处理中", "processing"}, {"已修复", "healed"}, {"失败", "failed"}, {"已跳过", "skipped"}, {"已忽略", "dismissed"}}},
	{Key: "severity", Label: "严重级别", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{"严重", "critical"}, {"高", "high"}, {"中", "medium"}, {"低", "low"}}},
	{Key: "has_plugin", Label: "关联插件", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
}

// PluginHandler 插件处理器
type PluginHandler struct {
	pluginSvc   *plugin.Service
	incidentSvc *plugin.IncidentService
}

type PluginHandlerDeps struct {
	PluginService   *plugin.Service
	IncidentService *plugin.IncidentService
}

// NewPluginHandler 创建插件处理器
func NewPluginHandler() *PluginHandler {
	return NewPluginHandlerWithDeps(PluginHandlerDeps{
		PluginService:   plugin.NewService(),
		IncidentService: plugin.NewIncidentService(),
	})
}

func NewPluginHandlerWithDeps(deps PluginHandlerDeps) *PluginHandler {
	return &PluginHandler{
		pluginSvc:   deps.PluginService,
		incidentSvc: deps.IncidentService,
	}
}

func (h *PluginHandler) Shutdown() {
	if h == nil || h.pluginSvc == nil {
		return
	}
	h.pluginSvc.Shutdown()
}

// GetPluginSearchSchema 获取插件搜索字段定义
func (h *PluginHandler) GetPluginSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": pluginSearchSchema})
}

// GetIncidentSearchSchema 获取工单搜索字段定义
func (h *PluginHandler) GetIncidentSearchSchema(c *gin.Context) {
	response.Success(c, gin.H{"fields": incidentSearchSchema})
}
