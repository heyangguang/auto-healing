package handler

import (
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ═══════════════════════════════════════════════════════════════
// Schema-Driven 动态搜索 — Handler 层工具
// ═══════════════════════════════════════════════════════════════

// SearchableField 可搜索字段声明（后端唯一真相源）
// 每个模块在 handler 中声明自己的 []SearchableField 作为白名单
type SearchableField struct {
	Key         string         `json:"key"`                   // 参数名（如 "name", "search"）
	Label       string         `json:"label"`                 // 界面标签（如 "名称"）
	Column      string         `json:"-"`                     // DB 列名（不暴露给前端）
	Columns     []string       `json:"-"`                     // 跨列搜索（search 字段：OR 多列）
	Type        string         `json:"type"`                  // "text" | "enum" | "boolean" | "dateRange"
	MatchModes  []string       `json:"match_modes"`           // ["fuzzy","exact"] 或 ["exact"]
	DefaultMode string         `json:"default_match_mode"`    // "fuzzy" 或 "exact"
	Placeholder string         `json:"placeholder,omitempty"` // 输入提示
	Description string         `json:"description,omitempty"` // 字段说明
	Options     []FilterOption `json:"options,omitempty"`     // 枚举选项
}

// FilterOption 枚举选项
type FilterOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ── 解析工具 ──

// GetStringFilter 从 gin.Context 提取字符串过滤参数
// 优先检查 field__exact 参数（精确匹配），否则回退到 field 参数（模糊匹配）
func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	// 优先精确匹配
	if exact := c.Query(field + "__exact"); exact != "" {
		return query.StringFilter{Value: exact, Exact: true}
	}
	// 回退模糊匹配
	if fuzzy := c.Query(field); fuzzy != "" {
		return query.StringFilter{Value: fuzzy, Exact: false}
	}
	return query.StringFilter{}
}

// ParseQueryFilters 根据白名单批量解析所有查询参数
// 只解析 type="text" 的字段，enum/boolean/dateRange 由各 handler 自行处理
func ParseQueryFilters(c *gin.Context, schema []SearchableField) map[string]query.StringFilter {
	filters := make(map[string]query.StringFilter)
	for _, field := range schema {
		if field.Type != "text" {
			continue
		}
		f := GetStringFilter(c, field.Key)
		if !f.IsEmpty() {
			filters[field.Key] = f
		}
	}
	return filters
}

// ApplyParsedFilters 将 ParseQueryFilters 返回的过滤器应用到 GORM 查询
// 根据 schema 中声明的 Column / Columns 自动选择单列或多列过滤
func ApplyParsedFilters(q *gorm.DB, schema []SearchableField, filters map[string]query.StringFilter) *gorm.DB {
	for _, field := range schema {
		if field.Type != "text" {
			continue
		}
		f, ok := filters[field.Key]
		if !ok || f.IsEmpty() {
			continue
		}
		// 如果声明了多列（Columns），使用OR多列搜索
		if len(field.Columns) > 0 {
			q = query.ApplyMultiStringFilter(q, field.Columns, f)
		} else if field.Column != "" {
			// 单列搜索
			q = query.ApplyStringFilter(q, field.Column, f)
		} else {
			// 默认用 Key 作为列名
			q = query.ApplyStringFilter(q, field.Key, f)
		}
	}
	return q
}

// BuildSchemaScopes 根据 schema 白名单从 gin.Context 解析独立字段过滤器
// 返回 GORM scopes 切片，可直接传给 repo/service 的 List 函数
// excludeKeys: 已由 handler 单独处理的字段 Key，不再重复生成 scope
func BuildSchemaScopes(c *gin.Context, schema []SearchableField, excludeKeys ...string) []func(*gorm.DB) *gorm.DB {
	filters := ParseQueryFilters(c, schema)
	// 移除 search（由各 handler 用 GetStringFilter("search") 单独处理）
	delete(filters, "search")
	// 移除已单独处理的字段
	for _, key := range excludeKeys {
		delete(filters, key)
	}
	if len(filters) == 0 {
		return nil
	}
	return []func(*gorm.DB) *gorm.DB{
		func(q *gorm.DB) *gorm.DB {
			return ApplyParsedFilters(q, schema, filters)
		},
	}
}
