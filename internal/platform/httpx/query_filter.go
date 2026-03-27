package httpx

import (
	"github.com/company/auto-healing/internal/pkg/query"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SearchableField struct {
	Key         string         `json:"key"`
	Label       string         `json:"label"`
	Column      string         `json:"-"`
	Columns     []string       `json:"-"`
	Type        string         `json:"type"`
	MatchModes  []string       `json:"match_modes"`
	DefaultMode string         `json:"default_match_mode"`
	Placeholder string         `json:"placeholder,omitempty"`
	Description string         `json:"description,omitempty"`
	Options     []FilterOption `json:"options,omitempty"`
}

type FilterOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func GetStringFilter(c *gin.Context, field string) query.StringFilter {
	if exact := c.Query(field + "__exact"); exact != "" {
		return query.StringFilter{Value: exact, Exact: true}
	}
	if fuzzy := c.Query(field); fuzzy != "" {
		return query.StringFilter{Value: fuzzy, Exact: false}
	}
	return query.StringFilter{}
}

func ParseQueryFilters(c *gin.Context, schema []SearchableField) map[string]query.StringFilter {
	filters := make(map[string]query.StringFilter)
	for _, field := range schema {
		if field.Type != "text" {
			continue
		}
		filter := GetStringFilter(c, field.Key)
		if !filter.IsEmpty() {
			filters[field.Key] = filter
		}
	}
	return filters
}

func ApplyParsedFilters(q *gorm.DB, schema []SearchableField, filters map[string]query.StringFilter) *gorm.DB {
	for _, field := range schema {
		if field.Type != "text" {
			continue
		}
		filter, ok := filters[field.Key]
		if !ok || filter.IsEmpty() {
			continue
		}
		if len(field.Columns) > 0 {
			q = query.ApplyMultiStringFilter(q, field.Columns, filter)
			continue
		}
		if field.Column != "" {
			q = query.ApplyStringFilter(q, field.Column, filter)
			continue
		}
		q = query.ApplyStringFilter(q, field.Key, filter)
	}
	return q
}

func BuildSchemaScopes(c *gin.Context, schema []SearchableField, excludeKeys ...string) []func(*gorm.DB) *gorm.DB {
	filters := ParseQueryFilters(c, schema)
	delete(filters, "search")
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
