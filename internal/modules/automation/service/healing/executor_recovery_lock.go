package healing

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

var recoveringInstances sync.Map

var ErrFlowInstanceRecoveryInProgress = errors.New("流程实例恢复进行中")

func startInstanceRecovery(instanceID uuid.UUID) bool {
	_, loaded := recoveringInstances.LoadOrStore(instanceID, struct{}{})
	return !loaded
}

func finishInstanceRecovery(instanceID uuid.UUID) {
	recoveringInstances.Delete(instanceID)
}
