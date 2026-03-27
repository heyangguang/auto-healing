package healing

import (
	"context"
	"fmt"

	"github.com/company/auto-healing/internal/modules/automation/model"
	"github.com/company/auto-healing/internal/pkg/logger"
)

func (e *FlowExecutor) executeHostExtractor(ctx context.Context, instance *model.FlowInstance, node *model.FlowNode) error {
	logger.Exec("NODE").Info("[%s] 执行主机提取节点", shortID(instance))

	prepared := e.prepareHostExtraction(node.Config)
	sourceValue := e.hostSourceValue(instance, prepared.SourceField)
	if sourceValue == "" {
		err := fmt.Errorf("源字段 %s 为空", prepared.SourceField)
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "提取主机失败", map[string]interface{}{
			"error":        err.Error(),
			"source_field": prepared.SourceField,
		})
		return err
	}

	hosts, err := e.extractHostsByMode(prepared, sourceValue)
	if err != nil {
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelError, "提取主机失败", map[string]interface{}{
			"error":        err.Error(),
			"extract_mode": prepared.ExtractMode,
			"source_field": prepared.SourceField,
			"source_value": sourceValue,
		})
		return err
	}
	if len(hosts) == 0 {
		err := fmt.Errorf("未提取到任何主机")
		e.logNode(ctx, instance.ID, node.ID, node.Type, model.LogLevelWarn, "未提取到主机", map[string]interface{}{
			"source_field": prepared.SourceField,
			"source_value": sourceValue,
			"extract_mode": prepared.ExtractMode,
		})
		return err
	}

	if err := e.storeExtractedHosts(ctx, instance, node.ID, prepared, sourceValue, hosts); err != nil {
		return err
	}
	logger.Exec("NODE").Info("[%s] 提取到 %d 个主机: %v", shortID(instance), len(hosts), hosts)
	return nil
}
