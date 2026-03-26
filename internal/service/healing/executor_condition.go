package healing

import (
	"context"

	"github.com/company/auto-healing/internal/model"
	"github.com/company/auto-healing/internal/pkg/logger"
	"github.com/google/uuid"
)

func (e *FlowExecutor) logNode(ctx context.Context, instanceID uuid.UUID, nodeID, nodeType, level, message string, details map[string]interface{}) {
	logEntry := &model.FlowExecutionLog{
		FlowInstanceID: instanceID,
		NodeID:         nodeID,
		NodeType:       nodeType,
		Level:          level,
		Message:        message,
		Details:        details,
	}
	if err := e.flowLogRepo.Create(ctx, logEntry); err != nil {
		logger.Exec("FLOW").Error("记录日志失败: %v", err)
	}
	e.eventBus.PublishNodeLog(instanceID, nodeID, nodeType, level, message, details)
}
