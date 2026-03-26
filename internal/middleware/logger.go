package middleware

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ANSI 颜色代码
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"

	redactedQueryValue = "REDACTED"
)

var sensitiveQueryKeys = []string{"token", "access_token", "refresh_token"}

// Logger 日志中间件 - 使用统一日志格式
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算耗时
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + redactRawQuery(raw)
		}

		// 格式化延迟
		latencyStr := formatLatency(latency)

		// 根据状态码选择颜色
		var statusColor string
		if statusCode >= 500 {
			statusColor = colorRed
		} else if statusCode >= 400 {
			statusColor = colorYellow
		} else {
			statusColor = colorGreen
		}

		// 带颜色的状态码
		coloredStatus := fmt.Sprintf("%s%d%s", statusColor, statusCode, colorReset)

		// 使用统一的日志格式
		// [API] 状态码 方法 路径 耗时 IP
		if statusCode >= 500 {
			logger.API("").Error("%s %s %s → %s | %s", coloredStatus, method, path, latencyStr, clientIP)
		} else if statusCode >= 400 {
			logger.API("").Warn("%s %s %s → %s | %s", coloredStatus, method, path, latencyStr, clientIP)
		} else {
			logger.API("").Info("%s %s %s → %s | %s", coloredStatus, method, path, latencyStr, clientIP)
		}

		// 如果是错误，输出额外信息
		if statusCode >= 400 && len(c.Errors) > 0 {
			logger.API("").Error("Error: %s", c.Errors.String())
		}
	}
}

func redactRawQuery(raw string) string {
	if raw == "" {
		return ""
	}

	values, err := url.ParseQuery(raw)
	if err == nil {
		return redactParsedQuery(values)
	}

	parts := strings.Split(raw, "&")
	for i, part := range parts {
		parts[i] = redactRawQueryPart(part)
	}
	return strings.Join(parts, "&")
}

func redactParsedQuery(values url.Values) string {
	for _, key := range sensitiveQueryKeys {
		if _, ok := values[key]; ok {
			values.Set(key, redactedQueryValue)
		}
	}
	return values.Encode()
}

func redactRawQueryPart(part string) string {
	key, _, found := strings.Cut(part, "=")
	if !found || !isSensitiveQueryKey(key) {
		return part
	}
	return key + "=" + redactedQueryValue
}

func isSensitiveQueryKey(key string) bool {
	for _, sensitiveKey := range sensitiveQueryKeys {
		if key == sensitiveKey {
			return true
		}
	}
	return false
}

// formatLatency 格式化延迟时间
func formatLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	} else if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	} else {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
