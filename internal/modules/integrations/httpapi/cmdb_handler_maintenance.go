package httpapi

import (
	"time"

	"github.com/company/auto-healing/internal/middleware"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const cmdbBatchMaintenanceLimit = 100

// EnterMaintenanceRequest 进入维护模式请求
type EnterMaintenanceRequest struct {
	Reason string `json:"reason" binding:"required"`
	EndAt  string `json:"end_at"`
}

// BatchMaintenanceRequest 批量维护请求
type BatchMaintenanceRequest struct {
	IDs    []string `json:"ids" binding:"required"`
	Reason string   `json:"reason" binding:"required"`
	EndAt  string   `json:"end_at"`
}

// BatchExitRequest 批量退出维护请求
type BatchExitRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// EnterMaintenance 进入维护模式
func (h *CMDBHandler) EnterMaintenance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	var req EnterMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: reason 必填")
		return
	}

	endAt, ok := parseCMDBMaintenanceEndAt(c, req.EndAt, "end_at 格式错误，请使用 RFC3339 格式")
	if !ok {
		return
	}
	if err := h.cmdbSvc.EnterMaintenance(c.Request.Context(), id, req.Reason, endAt, middleware.GetUsername(c)); err != nil {
		respondCMDBMaintenanceError(c, "进入维护模式失败", err)
		return
	}
	response.Message(c, "配置项已进入维护模式")
}

func parseCMDBMaintenanceEndAt(c *gin.Context, raw, message string) (*time.Time, bool) {
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.BadRequest(c, message)
		return nil, false
	}
	return &parsed, true
}

// ExitMaintenance 退出维护模式
func (h *CMDBHandler) ExitMaintenance(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}
	if err := h.cmdbSvc.ExitMaintenance(c.Request.Context(), id, "manual", middleware.GetUsername(c)); err != nil {
		respondCMDBMaintenanceError(c, "退出维护模式失败", err)
		return
	}
	response.Message(c, "配置项已恢复正常")
}

// GetMaintenanceLogs 获取维护日志
func (h *CMDBHandler) GetMaintenanceLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的ID格式")
		return
	}

	page, pageSize := parsePagination(c, 20)
	logs, total, err := h.cmdbSvc.GetMaintenanceLogs(c.Request.Context(), id, page, pageSize)
	if err != nil {
		respondInternalError(c, "CMDB", "获取维护日志失败", err)
		return
	}
	response.Success(c, map[string]interface{}{"data": logs, "total": total, "page": page, "page_size": pageSize})
}

// BatchEnterMaintenance 批量进入维护模式
func (h *CMDBHandler) BatchEnterMaintenance(c *gin.Context) {
	var req BatchMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	if !validateCMDBBatchIDs(c, req.IDs) {
		return
	}

	endAt, ok := parseCMDBMaintenanceEndAt(c, req.EndAt, "end_at 格式错误")
	if !ok {
		return
	}
	successCount := 0
	for _, idStr := range req.IDs {
		if id, err := uuid.Parse(idStr); err == nil {
			if err := h.cmdbSvc.EnterMaintenance(c.Request.Context(), id, req.Reason, endAt, middleware.GetUsername(c)); err == nil {
				successCount++
			}
		}
	}
	response.Success(c, batchCMDBResult(req.IDs, successCount))
}

// BatchExitMaintenance 批量退出维护模式
func (h *CMDBHandler) BatchExitMaintenance(c *gin.Context) {
	var req BatchExitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}
	if !validateCMDBBatchIDs(c, req.IDs) {
		return
	}

	successCount := 0
	for _, idStr := range req.IDs {
		if id, err := uuid.Parse(idStr); err == nil {
			if err := h.cmdbSvc.ExitMaintenance(c.Request.Context(), id, "manual", middleware.GetUsername(c)); err == nil {
				successCount++
			}
		}
	}
	response.Success(c, batchCMDBResult(req.IDs, successCount))
}

func validateCMDBBatchIDs(c *gin.Context, ids []string) bool {
	switch {
	case len(ids) == 0:
		response.BadRequest(c, "请选择配置项")
		return false
	case len(ids) > cmdbBatchMaintenanceLimit:
		response.BadRequest(c, "批量操作最多支持 100 个配置项")
		return false
	default:
		return true
	}
}

func batchCMDBResult(ids []string, successCount int) map[string]interface{} {
	return map[string]interface{}{
		"total":   len(ids),
		"success": successCount,
		"failed":  len(ids) - successCount,
	}
}
