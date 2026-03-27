package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func (e *FlowExecutor) executeCMDBValidator(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行 CMDB 校验节点", shortID(instance))

	prepared := e.prepareCMDBValidation(node.Config, instance.Context)
	if len(prepared.Hosts) == 0 {
		err := fmt.Errorf("未找到主机列表，input_key=%s", prepared.InputKey)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{
			"error":     err.Error(),
			"input_key": prepared.InputKey,
		})
		return err
	}

	validatedHosts, invalidHosts, err := e.validateHostsWithCMDB(ctx, instance, node, prepared)
	if err != nil {
		return err
	}
	if len(validatedHosts) == 0 {
		err := fmt.Errorf("没有任何主机通过 CMDB 验证")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "CMDB 验证失败", map[string]interface{}{
			"input_hosts":   prepared.Hosts,
			"invalid_hosts": invalidHosts,
		})
		return err
	}

	if err := e.storeCMDBValidationResult(ctx, instance, node.ID, prepared, validatedHosts, invalidHosts); err != nil {
		return err
	}
	logger.Exec("NODE").Info("[%s] CMDB 验证完成: %d/%d 个主机通过", shortID(instance), len(validatedHosts), len(prepared.Hosts))
	return nil
}
