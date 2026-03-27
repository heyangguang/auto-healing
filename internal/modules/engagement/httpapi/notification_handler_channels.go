package httpapi

import (
	notification "github.com/company/auto-healing/internal/modules/engagement/service/notification"
	"github.com/company/auto-healing/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListChannels 获取渠道列表
func (h *NotificationHandler) ListChannels(c *gin.Context) {
	page, pageSize := parsePagination(c, 20)
	channelType := c.Query("type")
	name := GetStringFilter(c, "name")

	channels, total, err := h.svc.ListChannels(c.Request.Context(), page, pageSize, channelType, name)
	if err != nil {
		respondInternalError(c, "NOTIFY", "获取通知渠道列表失败", err)
		return
	}
	response.List(c, channels, total, page, pageSize)
}

// CreateChannel 创建渠道
func (h *NotificationHandler) CreateChannel(c *gin.Context) {
	var req notification.CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	channel, err := h.svc.CreateChannel(c.Request.Context(), req)
	if err != nil {
		writeNotificationMutationError(c, err, "渠道不存在", "创建通知渠道失败")
		return
	}
	response.Created(c, channel)
}

// GetChannel 获取渠道详情
func (h *NotificationHandler) GetChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	channel, err := h.svc.GetChannel(c.Request.Context(), id)
	if err != nil {
		writeNotificationLookupError(c, err, "渠道不存在", "获取通知渠道失败")
		return
	}
	response.Success(c, channel)
}

// UpdateChannel 更新渠道
func (h *NotificationHandler) UpdateChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	var req notification.UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	channel, err := h.svc.UpdateChannel(c.Request.Context(), id, req)
	if err != nil {
		writeNotificationMutationError(c, err, "渠道不存在", "更新通知渠道失败")
		return
	}
	response.Success(c, channel)
}

// DeleteChannel 删除渠道
func (h *NotificationHandler) DeleteChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	if err := h.svc.DeleteChannel(c.Request.Context(), id); err != nil {
		writeNotificationMutationError(c, err, "渠道不存在", "删除通知渠道失败")
		return
	}
	response.Message(c, "删除成功")
}

// TestChannel 测试渠道
func (h *NotificationHandler) TestChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的 ID")
		return
	}

	if err := h.svc.TestChannel(c.Request.Context(), id); err != nil {
		writeNotificationMutationError(c, err, "渠道不存在", "测试通知渠道失败")
		return
	}
	response.Message(c, "测试成功")
}
