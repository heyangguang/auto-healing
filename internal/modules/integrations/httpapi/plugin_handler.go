package httpapi

import (
	"github.com/company/auto-healing/internal/modules/integrations/service/plugin"
	"github.com/company/auto-healing/internal/pkg/response"
	incidentrepo "github.com/company/auto-healing/internal/platform/repository/incident"
	"github.com/gin-gonic/gin"
)

// ══════ 插件搜索白名单 ══════
var pluginSearchSchema = []SearchableField{
	{Key: "name", Label: "名称", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "插件名称", Column: "name"},
	{Key: "description", Label: "描述", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "插件描述", Column: "description"},
	{Key: "type", Label: "类型", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "ITSM", Value: "itsm"}, {Label: "CMDB", Value: "cmdb"}}},
	{Key: "status", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "活跃", Value: "active"}, {Label: "停用", Value: "inactive"}, {Label: "异常", Value: "error"}}},
}

// ══════ 工单搜索白名单 ══════
var incidentSearchSchema = []SearchableField{
	{Key: "title", Label: "标题", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Placeholder: "事件标题", Column: "title"},
	{Key: "source_plugin_name", Label: "来源插件", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "fuzzy", Column: "source_plugin_name", Placeholder: "插件名称"},
	{Key: "external_id", Label: "外部ID", Type: "text", MatchModes: []string{"fuzzy", "exact"}, DefaultMode: "exact", Column: "external_id", Placeholder: "外部工单ID"},
	{Key: "status", Label: "状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "待处理", Value: "open"}, {Label: "已关闭", Value: "closed"}}},
	{Key: "healing_status", Label: "自愈状态", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "待处理", Value: "pending"}, {Label: "处理中", Value: "processing"}, {Label: "已修复", Value: "healed"}, {Label: "失败", Value: "failed"}, {Label: "已跳过", Value: "skipped"}, {Label: "已忽略", Value: "dismissed"}}},
	{Key: "severity", Label: "严重级别", Type: "enum", MatchModes: []string{"exact"}, DefaultMode: "exact", Options: []FilterOption{{Label: "严重", Value: "critical"}, {Label: "高", Value: "high"}, {Label: "中", Value: "medium"}, {Label: "低", Value: "low"}}},
	{Key: "has_plugin", Label: "关联插件", Type: "boolean", MatchModes: []string{"exact"}, DefaultMode: "exact"},
}

// PluginHandler 插件处理器
type PluginHandler struct {
	pluginSvc           *plugin.Service
	incidentSvc         *plugin.IncidentService
	solutionTemplateSvc *plugin.SolutionTemplateService
}

type PluginHandlerDeps struct {
	PluginService           *plugin.Service
	IncidentService         *plugin.IncidentService
	SolutionTemplateService *plugin.SolutionTemplateService
	IncidentRepo            *incidentrepo.IncidentRepository
}

func NewPluginHandlerWithDeps(deps PluginHandlerDeps) *PluginHandler {
	switch {
	case deps.PluginService == nil:
		panic("integrations plugin handler requires plugin service")
	case deps.IncidentService == nil:
		panic("integrations plugin handler requires incident service")
	case deps.SolutionTemplateService == nil:
		panic("integrations plugin handler requires solution template service")
	}
	return &PluginHandler{
		pluginSvc:           deps.PluginService,
		incidentSvc:         deps.IncidentService,
		solutionTemplateSvc: deps.SolutionTemplateService,
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
	response.Success(c, pluginSearchSchema)
}

// GetIncidentSearchSchema 获取工单搜索字段定义
func (h *PluginHandler) GetIncidentSearchSchema(c *gin.Context) {
	response.Success(c, incidentSearchSchema)
}
