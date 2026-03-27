package handler

import (
	"github.com/company/auto-healing/internal/pkg/query"
	platformhttp "github.com/company/auto-healing/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ═══════════════════════════════════════════════════════════════
// Schema-Driven 动态搜索 — Handler 层工具
// ═══════════════════════════════════════════════════════════════

// SearchableField 可搜索字段声明（后端唯一真相源）
// 每个模块在 handler 中声明自己的 []SearchableField 作为白名单
type SearchableField = platformhttp.SearchableField

type FilterOption = platformhttp.FilterOption

// ── 解析工具 ──

// GetStringFilter 从 gin.Context 提取字符串过滤参数
// 优先检查 field__exact 参数（精确匹配），否则回退到 field 参数（模糊匹配）
func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	return platformhttp.GetStringFilter(c, field)
}

// ParseQueryFilters 根据白名单批量解析所有查询参数
// 只解析 type="text" 的字段，enum/boolean/dateRange 由各 handler 自行处理
func ParseQueryFilters(c *gin.Context, schema []SearchableField) map[string]query.StringFilter {
	return platformhttp.ParseQueryFilters(c, schema)
}

// ApplyParsedFilters 将 ParseQueryFilters 返回的过滤器应用到 GORM 查询
// 根据 schema 中声明的 Column / Columns 自动选择单列或多列过滤
func ApplyParsedFilters(q *gorm.DB, schema []SearchableField, filters map[string]query.StringFilter) *gorm.DB {
	return platformhttp.ApplyParsedFilters(q, schema, filters)
}

// BuildSchemaScopes 根据 schema 白名单从 gin.Context 解析独立字段过滤器
// 返回 GORM scopes 切片，可直接传给 repo/service 的 List 函数
// excludeKeys: 已由 handler 单独处理的字段 Key，不再重复生成 scope
func BuildSchemaScopes(c *gin.Context, schema []SearchableField, excludeKeys ...string) []func(*gorm.DB) *gorm.DB {
	return platformhttp.BuildSchemaScopes(c, schema, excludeKeys...)
}
