package plugin

import "fmt"

func validatePluginMutation(pluginType string, syncEnabled bool, syncIntervalMinutes, maxFailures int) error {
	if pluginType != "itsm" && pluginType != "cmdb" {
		return fmt.Errorf("不支持的插件类型: %s", pluginType)
	}
	if maxFailures < 0 {
		return fmt.Errorf("最大连续失败次数不能为负数")
	}
	if syncEnabled && syncIntervalMinutes < 1 {
		return fmt.Errorf("同步间隔最小为1分钟")
	}
	return nil
}
