package plugin

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

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

func validatePluginConfiguration(pluginType string, config, fieldMapping, syncFilter model.JSON) error {
	if err := validatePluginHTTPConfig(pluginType, config); err != nil {
		return err
	}
	if err := validatePluginFieldMapping(pluginType, fieldMapping); err != nil {
		return err
	}
	if _, err := ParseSyncFilter(syncFilter); err != nil {
		return fmt.Errorf("sync_filter 无效: %w", err)
	}
	return nil
}

func validatePluginHTTPConfig(pluginType string, config model.JSON) error {
	if len(config) == 0 {
		return fmt.Errorf("插件配置不能为空")
	}
	rawURL, _ := config["url"].(string)
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("config.url 不能为空")
	}
	if err := validateHTTPURL(rawURL, "config.url"); err != nil {
		return err
	}
	if err := validatePluginAuthConfig(config); err != nil {
		return err
	}
	if value, ok := config["response_data_path"]; ok {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("config.response_data_path 必须是字符串")
		}
	}
	if value, ok := config["since_param"]; ok {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("config.since_param 必须是字符串")
		}
	}
	if value, ok := config["extra_params"]; ok {
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("config.extra_params 必须是对象")
		}
	}
	if pluginType == "itsm" {
		if err := validateIncidentWritebackConfig(config); err != nil {
			return err
		}
	}
	return nil
}

func validateHTTPURL(raw, field string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("%s 不是合法 URL: %w", field, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s 仅支持 http 或 https", field)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s 必须包含 host", field)
	}
	return nil
}

func validatePluginAuthConfig(config model.JSON) error {
	authType, _ := config["auth_type"].(string)
	switch authType {
	case "", "none":
		return nil
	case "basic":
		if strings.TrimSpace(stringField(config, "username")) == "" {
			return fmt.Errorf("config.username 不能为空")
		}
		if strings.TrimSpace(stringField(config, "password")) == "" {
			return fmt.Errorf("config.password 不能为空")
		}
	case "bearer":
		if strings.TrimSpace(stringField(config, "token")) == "" {
			return fmt.Errorf("config.token 不能为空")
		}
	case "api_key":
		if strings.TrimSpace(stringField(config, "api_key")) == "" {
			return fmt.Errorf("config.api_key 不能为空")
		}
		if value, ok := config["api_key_header"]; ok {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("config.api_key_header 必须是字符串")
			}
		}
	default:
		return fmt.Errorf("config.auth_type 不支持: %s", authType)
	}
	return nil
}

func validateIncidentWritebackConfig(config model.JSON) error {
	rawURL, _ := config["close_incident_url"].(string)
	if rawURL == "" {
		return nil
	}
	if err := validateHTTPURL(strings.ReplaceAll(rawURL, "{external_id}", "INC-1"), "config.close_incident_url"); err != nil {
		return err
	}
	method := strings.ToUpper(stringField(config, "close_incident_method"))
	if method == "" {
		return nil
	}
	switch method {
	case "POST", "PUT", "PATCH":
		return nil
	default:
		return fmt.Errorf("config.close_incident_method 仅支持 POST、PUT 或 PATCH")
	}
}

func validatePluginFieldMapping(pluginType string, fieldMapping model.JSON) error {
	if len(fieldMapping) == 0 {
		return nil
	}
	allowedSections := map[string]bool{
		"incident_mapping": pluginType == "itsm",
		"cmdb_mapping":     pluginType == "cmdb",
	}
	for section, raw := range fieldMapping {
		if ok := allowedSections[section]; !ok {
			return fmt.Errorf("field_mapping.%s 不适用于 %s 插件", section, pluginType)
		}
		mapping, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("field_mapping.%s 必须是对象", section)
		}
		for field, value := range mapping {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("field_mapping.%s.%s 必须是字符串", section, field)
			}
		}
	}
	return nil
}

func stringField(values model.JSON, key string) string {
	value, _ := values[key].(string)
	return value
}
