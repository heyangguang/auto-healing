package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/model"
)

// HTTPClient HTTP 客户端，用于与外部系统通信
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient 创建 HTTP 客户端
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TestConnection 测试连接
func (c *HTTPClient) TestConnection(ctx context.Context, config model.JSON) error {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("配置缺少 url 字段")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证
	c.addAuth(req, config)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("连接测试返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// FetchData 拉取数据
func (c *HTTPClient) FetchData(ctx context.Context, config model.JSON, since time.Time) ([]map[string]interface{}, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("配置缺少 url 字段")
	}

	// 构建查询参数
	params := []string{}

	// 添加额外查询参数（extra_params）
	if extraParams, ok := config["extra_params"].(map[string]interface{}); ok {
		for k, v := range extraParams {
			if str, ok := v.(string); ok {
				params = append(params, fmt.Sprintf("%s=%s", k, str))
			}
		}
	}

	// 添加时间参数（如果配置了）
	if timeParam, ok := config["since_param"].(string); ok && timeParam != "" {
		params = append(params, fmt.Sprintf("%s=%s", timeParam, since.Format(time.RFC3339)))
	}

	// 拼接查询参数到 URL
	if len(params) > 0 {
		separator := "?"
		if strings.Contains(url, "?") {
			separator = "&"
		}
		url = url + separator + strings.Join(params, "&")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证
	c.addAuth(req, config)

	// 设置 Accept header
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("请求返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	return c.parseResponse(body, config)
}

// CloseIncident 关闭工单
func (c *HTTPClient) CloseIncident(ctx context.Context, config model.JSON, closeData map[string]interface{}) error {
	closeURL, ok := config["close_incident_url"].(string)
	if !ok || closeURL == "" {
		return fmt.Errorf("未配置关闭工单接口 (close_incident_url)")
	}

	// 替换 URL 中的占位符
	if externalID, ok := closeData["external_id"].(string); ok {
		closeURL = strings.ReplaceAll(closeURL, "{external_id}", externalID)
	}

	jsonBody, err := json.Marshal(closeData)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	method := "POST"
	if m, ok := config["close_incident_method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	req, err := http.NewRequestWithContext(ctx, method, closeURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证
	c.addAuth(req, config)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("关闭工单返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// addAuth 添加认证信息
func (c *HTTPClient) addAuth(req *http.Request, config model.JSON) {
	authType, _ := config["auth_type"].(string)

	switch authType {
	case "basic":
		username, _ := config["username"].(string)
		password, _ := config["password"].(string)
		auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		req.Header.Set("Authorization", "Basic "+auth)

	case "bearer":
		token, _ := config["token"].(string)
		req.Header.Set("Authorization", "Bearer "+token)

	case "api_key":
		apiKey, _ := config["api_key"].(string)
		headerName, _ := config["api_key_header"].(string)
		if headerName == "" {
			headerName = "X-API-Key"
		}
		req.Header.Set(headerName, apiKey)
	}
}

// parseResponse 解析响应数据
func (c *HTTPClient) parseResponse(body []byte, config model.JSON) ([]map[string]interface{}, error) {
	// 尝试解析为数组
	var arrayResult []map[string]interface{}
	if err := json.Unmarshal(body, &arrayResult); err == nil {
		return arrayResult, nil
	}

	// 尝试解析为对象，并从指定路径提取数据
	var objResult map[string]interface{}
	if err := json.Unmarshal(body, &objResult); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查是否配置了数据路径（如 "data.items"）
	if dataPath, ok := config["response_data_path"].(string); ok && dataPath != "" {
		parts := strings.Split(dataPath, ".")
		current := objResult

		for i, part := range parts {
			if val, ok := current[part]; ok {
				if i == len(parts)-1 {
					// 最后一部分，应该是数组
					if arr, ok := val.([]interface{}); ok {
						result := make([]map[string]interface{}, 0, len(arr))
						for _, item := range arr {
							if m, ok := item.(map[string]interface{}); ok {
								result = append(result, m)
							}
						}
						return result, nil
					}
				} else {
					// 中间部分，应该是对象
					if obj, ok := val.(map[string]interface{}); ok {
						current = obj
					} else {
						break
					}
				}
			}
		}
	}

	// 如果有 data 字段且是数组
	if data, ok := objResult["data"].([]interface{}); ok {
		result := make([]map[string]interface{}, 0, len(data))
		for _, item := range data {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result, nil
	}

	// 如果有 items 字段且是数组
	if items, ok := objResult["items"].([]interface{}); ok {
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("无法从响应中提取数据数组")
}
