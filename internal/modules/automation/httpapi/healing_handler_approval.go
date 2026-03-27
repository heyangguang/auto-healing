package httpapi

import (
	"context"
	"errors"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/company/auto-healing/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ========== ApprovalTask 相关 ==========

// ListApprovals 获取审批任务列表
func (h *HealingHandler) ListApprovals(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	status := c.Query("status")

	var flowInstanceID *uuid.UUID
	if str := c.Query("flow_instance_id"); str != "" {
		if val, err := uuid.Parse(str); err == nil {
			flowInstanceID = &val
		}
	}

	tasks, total, err := h.approvalRepo.List(c.Request.Context(), page, pageSize, flowInstanceID, status)
	if err != nil {
		response.InternalError(c, "获取审批任务列表失败")
		return
	}

	response.List(c, tasks, total, page, pageSize)
}

// ListPendingApprovals 获取待审批任务列表
// 支持 Query 参数：node_name（模糊搜索 node_id）、date_from、date_to
func (h *HealingHandler) ListPendingApprovals(c *gin.Context) {
	page := getQueryInt(c, "page", 1)
	pageSize := getQueryInt(c, "page_size", 20)
	nodeName := c.Query("node_name")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")

	tasks, total, err := h.approvalRepo.ListPending(c.Request.Context(), page, pageSize, nodeName, dateFrom, dateTo)
	if err != nil {
		response.InternalError(c, "获取待审批任务列表失败")
		return
	}

	response.List(c, tasks, total, page, pageSize)
}

// GetApproval 获取审批任务详情
func (h *HealingHandler) GetApproval(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的审批任务ID")
		return
	}

	task, err := h.approvalRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "审批任务不存在")
		return
	}

	response.Success(c, task)
}

// ApproveTask 批准审批任务
func (h *HealingHandler) ApproveTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的审批任务ID")
		return
	}
	req, ok := parseApprovalRequest(c)
	if !ok {
		return
	}
	userID, ok := requireCurrentUserID(c)
	if !ok {
		return
	}
	task, ok := h.loadPendingApprovalTask(c, id)
	if !ok {
		return
	}
	if err := h.approvalRepo.Approve(c.Request.Context(), id, *userID, req.Comment); err != nil {
		if errors.Is(err, repository.ErrApprovalTaskNotPending) {
			response.Conflict(c, "审批任务已处理，请刷新后查看最新状态")
			return
		}
		response.InternalError(c, "批准操作失败")
		return
	}
	h.executor.Go(func(rootCtx context.Context) {
		h.resumeApprovedFlow(rootCtx, task)
	})
	response.Message(c, "审批已通过")
}

func parseApprovalRequest(c *gin.Context) (*ApproveRequest, bool) {
	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return nil, false
	}
	return &req, true
}

func requireCurrentUserID(c *gin.Context) (*uuid.UUID, bool) {
	userID := getCurrentUserID(c)
	if userID == nil {
		response.Unauthorized(c, "未授权")
		return nil, false
	}
	return userID, true
}

func (h *HealingHandler) loadPendingApprovalTask(c *gin.Context, id uuid.UUID) (*model.ApprovalTask, bool) {
	task, err := h.approvalRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "审批任务不存在")
		return nil, false
	}
	if task.FlowInstance != nil && task.FlowInstance.Status != model.FlowInstanceStatusWaitingApproval {
		response.Conflict(c, "流程实例当前不处于待审批状态，无法继续审批")
		return nil, false
	}
	return task, true
}

func (h *HealingHandler) resumeApprovedFlow(rootCtx context.Context, task *model.ApprovalTask) {
	defer func() {
		if r := recover(); r != nil {
			logger.Exec("APPROVAL").Error("恢复流程 panic: %v", r)
		}
	}()
	resumeCtx := rootCtx
	if task.TenantID != nil {
		resumeCtx = repository.WithTenantID(resumeCtx, *task.TenantID)
	}
	if err := h.executor.ResumeAfterApproval(resumeCtx, task.FlowInstanceID, true); err != nil {
		logger.Exec("APPROVAL").Error("审批通过后恢复流程失败: %v", err)
	}
}

// RejectTask 拒绝审批任务
func (h *HealingHandler) RejectTask(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的审批任务ID")
		return
	}

	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	userID := getCurrentUserID(c)
	if userID == nil {
		response.Unauthorized(c, "未授权")
		return
	}

	// 获取审批任务信息
	task, err := h.approvalRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "审批任务不存在")
		return
	}

	if err := h.approvalRepo.Reject(c.Request.Context(), id, *userID, req.Comment); err != nil {
		if errors.Is(err, repository.ErrApprovalTaskNotPending) {
			response.Conflict(c, "审批任务已处理，请刷新后查看最新状态")
			return
		}
		response.InternalError(c, "拒绝操作失败")
		return
	}

	h.executor.Go(func(rootCtx context.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Exec("APPROVAL").Error("拒绝后恢复流程 panic: %v", r)
			}
		}()
		resumeCtx := rootCtx
		if task.TenantID != nil {
			resumeCtx = repository.WithTenantID(resumeCtx, *task.TenantID)
		}
		if err := h.executor.ResumeAfterApproval(resumeCtx, task.FlowInstanceID, false); err != nil {
			logger.Exec("APPROVAL").Error("审批拒绝后恢复流程失败: %v", err)
		}
	})

	response.Message(c, "审批已拒绝")
}
