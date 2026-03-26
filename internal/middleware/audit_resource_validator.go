package middleware

import (
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/service"
	"github.com/gin-gonic/gin"
)

// ValidateAuditResourceTypes 启动时校验：扫描所有路由，推断可能产出的 resource_type，
// 与字典种子对比，发现字典缺失的打 WARN 日志。
//
// 目的：避免新增模块时遗漏字典种子同步，导致前端审计日志显示英文 resource_type。
func ValidateAuditResourceTypes(router *gin.Engine) {
	// 1. 收集字典种子中已有的 resource_type（key 集合）
	dictKeys := make(map[string]bool)
	for _, d := range service.AllDictionarySeeds {
		if d.DictType == "audit_resource_tenant" || d.DictType == "audit_resource_platform" {
			dictKeys[d.DictKey] = true
		}
	}

	// 2. 从路由表推断所有可能的 resource_type
	routeResourceTypes := make(map[string]string) // resource_type → 示例路径
	for _, route := range router.Routes() {
		// 只关注写操作
		if route.Method == "GET" || route.Method == "OPTIONS" || route.Method == "HEAD" {
			continue
		}
		path := route.Path
		// 跳过不审计的路由
		if shouldSkipAudit(path) {
			continue
		}

		// 调用与审计中间件完全相同的推断逻辑
		_, resourceType := inferActionAndResource(route.Method, path)
		if resourceType == "" || resourceType == "unknown" {
			continue
		}
		if _, exists := routeResourceTypes[resourceType]; !exists {
			routeResourceTypes[resourceType] = route.Method + " " + path
		}
	}

	// 3. 对比 — 找出路由中产出但字典中缺失的 resource_type
	var missing []string
	for rt, examplePath := range routeResourceTypes {
		if !dictKeys[rt] {
			missing = append(missing, rt+" (例: "+examplePath+")")
		}
	}

	if len(missing) > 0 {
		logger.API("AUDIT").Warn("以下审计资源类型在字典种子中缺失中文标签，请更新 dictionary_seeds_extra.go 的 auditResourceSeeds() | count=%d missing=%v", len(missing), missing)
	} else {
		logger.API("AUDIT").Info("审计资源类型字典校验通过 | route_types=%d dict_entries=%d", len(routeResourceTypes), len(dictKeys))
	}
}
