package healing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFlowExecutorCancelInvokesStoredCancelFunc(t *testing.T) {
	instanceID := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	runningFlowCancels.Store(instanceID, cancel)
	t.Cleanup(func() { runningFlowCancels.Delete(instanceID) })

	executor := &FlowExecutor{}
	executor.Cancel(instanceID)

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Cancel() did not invoke stored cancel func")
	}
}
