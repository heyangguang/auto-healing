package query

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ═══════════════════════════════════════════════════════════════
// 通用查询过滤器 — 被 handler 和 repository 共享
// ═══════════════════════════════════════════════════════════════

// StringFilter 携带匹配模式的过滤参数
type StringFilter struct {
	Value string
	Exact bool // true=精确匹配 (=), false=模糊匹配 (ILIKE)
}

// IsEmpty 检查过滤参数是否为空
func (f StringFilter) IsEmpty() bool {
	return f.Value == ""
}

// ApplyStringFilter 应用单个字符串过滤到 GORM query
// exact=true  → WHERE column = ?
// exact=false → WHERE column ILIKE '%xxx%'
func ApplyStringFilter(q *gorm.DB, column string, filter StringFilter) *gorm.DB {
	if filter.IsEmpty() {
		return q
	}
	if filter.Exact {
		return q.Where(fmt.Sprintf("%s = ?", column), filter.Value)
	}
	return q.Where(fmt.Sprintf("%s ILIKE ?", column), "%"+filter.Value+"%")
}

// ApplyMultiStringFilter 跨多列搜索（如 search 字段：名称 OR 描述）
// exact=true  → WHERE (col1 = ? OR col2 = ?)
// exact=false → WHERE (col1 ILIKE ? OR col2 ILIKE ?)
func ApplyMultiStringFilter(q *gorm.DB, columns []string, filter StringFilter) *gorm.DB {
	if filter.IsEmpty() || len(columns) == 0 {
		return q
	}

	conditions := make([]string, len(columns))
	args := make([]interface{}, len(columns))

	for i, col := range columns {
		if filter.Exact {
			conditions[i] = fmt.Sprintf("%s = ?", col)
			args[i] = filter.Value
		} else {
			conditions[i] = fmt.Sprintf("%s ILIKE ?", col)
			args[i] = "%" + filter.Value + "%"
		}
	}

	whereClause := "(" + strings.Join(conditions, " OR ") + ")"
	return q.Where(whereClause, args...)
}
