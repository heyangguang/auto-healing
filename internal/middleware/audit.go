package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuditMiddleware 审计中间件 — 自动记录写操作的审计日志
func AuditMiddleware() gin.HandlerFunc {
	repo := repository.NewAuditLogRepository()

	return func(c *gin.Context) {
		// 只记录写操作
		method := c.Request.Method
		if method == "GET" || method == "OPTIONS" || method == "HEAD" {
			c.Next()
			return
		}

		// 跳过认证路由（登录/登出由 auth handler 自行记录更精确）
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/auth/login") || strings.HasPrefix(path, "/api/v1/auth/refresh") {
			c.Next()
			return
		}

		// 读取请求体（供后续记录）
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 记录开始时间
		startTime := time.Now()

		// 执行后续处理
		c.Next()

		// 异步记录审计日志
		go func() {
			defer func() {
				if r := recover(); r != nil {
					zap.L().Error("审计日志记录失败 (panic)", zap.Any("error", r))
				}
			}()

			// 从上下文提取用户信息
			userIDStr := GetUserID(c)
			username := GetUsername(c)

			var userID *uuid.UUID
			if userIDStr != "" {
				if uid, err := uuid.Parse(userIDStr); err == nil {
					userID = &uid
				}
			}

			// 推断操作类型和资源类型
			action, resourceType := inferActionAndResource(method, path)

			// 推断资源 ID
			var resourceID *uuid.UUID
			if rid := extractResourceID(path); rid != nil {
				resourceID = rid
			}

			// 判断状态
			status := "success"
			errorMsg := ""
			statusCode := c.Writer.Status()
			if statusCode >= 400 {
				status = "failed"
			}

			// 限制请求体大小（避免存储过大数据）
			var bodyJSON model.JSON
			if len(requestBody) > 0 && len(requestBody) < 10240 {
				_ = json.Unmarshal(requestBody, &bodyJSON)
			}

			auditLog := &model.AuditLog{
				UserID:         userID,
				Username:       username,
				IPAddress:      c.ClientIP(),
				UserAgent:      c.Request.UserAgent(),
				Action:         action,
				ResourceType:   resourceType,
				ResourceID:     resourceID,
				RequestMethod:  method,
				RequestPath:    path,
				RequestBody:    bodyJSON,
				ResponseStatus: &statusCode,
				Status:         status,
				ErrorMessage:   errorMsg,
				CreatedAt:      startTime,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := repo.Create(ctx, auditLog); err != nil {
				zap.L().Error("审计日志写入失败", zap.Error(err))
			}
		}()
	}
}

// inferActionAndResource 从 HTTP 方法和路径推断操作类型和资源类型
func inferActionAndResource(method, path string) (action, resourceType string) {
	// 去掉 /api/v1/ 前缀
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	// 获取资源类型（第一段）
	if len(parts) > 0 {
		resourceType = parts[0]
	}

	// 基于 HTTP 方法推断操作
	switch method {
	case "POST":
		action = "create"
		// 检查特殊后缀操作
		if len(parts) >= 3 {
			lastPart := parts[len(parts)-1]
			switch lastPart {
			case "activate":
				action = "activate"
			case "deactivate":
				action = "deactivate"
			case "test":
				action = "test"
			case "sync":
				action = "sync"
			case "execute":
				action = "execute"
			case "enable":
				action = "enable"
			case "disable":
				action = "disable"
			case "approve":
				action = "approve"
			case "reject":
				action = "reject"
			case "cancel":
				action = "cancel"
			case "retry":
				action = "retry"
			case "reset-password":
				action = "reset_password"
			case "trigger":
				action = "trigger"
			case "confirm-review":
				action = "confirm_review"
			case "dry-run", "dry-run-stream":
				action = "dry_run"
			case "reset-scan", "batch-reset-scan":
				action = "reset_scan"
			case "reset-status":
				action = "reset_status"
			case "send":
				action = "send"
			case "preview":
				action = "preview"
			case "ready":
				action = "ready"
			case "offline":
				action = "offline"
			case "scan":
				action = "scan"
			case "maintenance":
				action = "maintenance"
			case "resume":
				action = "resume"
			}
		}

		// 批量操作
		if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-1], "batch") {
			action = "batch_" + action
		}
	case "PUT":
		action = "update"
		// 特殊：角色分配
		if len(parts) >= 3 {
			lastPart := parts[len(parts)-1]
			switch lastPart {
			case "roles":
				action = "assign_role"
			case "permissions":
				action = "assign_permission"
			case "variables":
				action = "update_variables"
			case "workspaces":
				action = "assign_workspace"
			}
		}
	case "DELETE":
		action = "delete"
	case "PATCH":
		action = "patch"
	}

	return action, resourceType
}

// extractResourceID 从 URL 路径中提取资源 ID
func extractResourceID(path string) *uuid.UUID {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	parts := strings.Split(trimmed, "/")

	// 遍历路径片段，查找 UUID
	for _, part := range parts {
		if uid, err := uuid.Parse(part); err == nil {
			return &uid
		}
	}
	return nil
}
