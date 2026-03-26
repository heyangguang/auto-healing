package healing

import "github.com/google/uuid"

// Cancel 请求取消指定流程实例的运行上下文。
func (e *FlowExecutor) Cancel(instanceID uuid.UUID) {
	cancel, ok := runningFlowCancels.Load(instanceID)
	if !ok {
		return
	}
	cancel.(func())()
}
