package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/company/auto-healing/internal/modules/integrations/model"
)

// HTTPClient HTTP 客户端，用于与外部系统通信
type HTTPClient struct {
	client *http.Client
}

type IncidentWritebackHTTPResult struct {
	RequestMethod string
	RequestURL    string
	ResponseBody  string
	StatusCode    int
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
	if err := c.addAuth(req, config); err != nil {
		return err
	}

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
	url, err := configURL(config)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.buildRequestURL(url, config, since), nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if err := c.addAuth(req, config); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	body, err := c.doJSONRequest(req, "请求")
	if err != nil {
		return nil, err
	}
	return c.parseResponse(body, config)
}

func configURL(config model.JSON) (string, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return "", fmt.Errorf("配置缺少 url 字段")
	}
	return url, nil
}

func (c *HTTPClient) buildRequestURL(url string, config model.JSON, since time.Time) string {
	params := c.buildQueryParams(config, since)
	if len(params) == 0 {
		return url
	}

	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}
	return url + separator + params.Encode()
}

func (c *HTTPClient) buildQueryParams(config model.JSON, since time.Time) url.Values {
	params := url.Values{}
	if extraParams, ok := config["extra_params"].(map[string]interface{}); ok {
		for key, value := range extraParams {
			if str, ok := value.(string); ok {
				params.Set(key, str)
			}
		}
	}
	if timeParam, ok := config["since_param"].(string); ok && timeParam != "" {
		params.Set(timeParam, since.Format(time.RFC3339))
	}
	return params
}

func (c *HTTPClient) doJSONRequest(req *http.Request, action string) ([]byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s失败: %w", action, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s返回错误状态码 %d: %s", action, resp.StatusCode, string(body))
	}
	return body, nil
}

// CloseIncident 关闭工单
func (c *HTTPClient) CloseIncident(
	ctx context.Context,
	config model.JSON,
	closeURL string,
	method string,
	closeData map[string]interface{},
) (*IncidentWritebackHTTPResult, error) {
	jsonBody, err := json.Marshal(closeData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, closeURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	if err := c.addAuth(req, config); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doIncidentWritebackRequest(req)
}

func (c *HTTPClient) doIncidentWritebackRequest(req *http.Request) (*IncidentWritebackHTTPResult, error) {
	result := &IncidentWritebackHTTPResult{
		RequestMethod: req.Method,
		RequestURL:    req.URL.String(),
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return result, fmt.Errorf("关闭工单失败: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return result, fmt.Errorf("读取响应失败: %w", readErr)
	}
	result.StatusCode = resp.StatusCode
	result.ResponseBody = string(body)
	if resp.StatusCode >= 400 {
		return result, fmt.Errorf("关闭工单返回错误状态码 %d: %s", resp.StatusCode, result.ResponseBody)
	}
	return result, nil
}

// addAuth 添加认证信息
func (c *HTTPClient) addAuth(req *http.Request, config model.JSON) error {
	authType, _ := config["auth_type"].(string)

	switch authType {
	case "", "none":
		return nil
	case "basic":
		username, _ := config["username"].(string)
		password, _ := config["password"].(string)
		auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		req.Header.Set("Authorization", "Basic "+auth)
		return nil

	case "bearer":
		token, _ := config["token"].(string)
		req.Header.Set("Authorization", "Bearer "+token)
		return nil

	case "api_key":
		apiKey, _ := config["api_key"].(string)
		headerName, _ := config["api_key_header"].(string)
		if headerName == "" {
			headerName = "X-API-Key"
		}
		req.Header.Set(headerName, apiKey)
		return nil
	}
	return fmt.Errorf("不支持的认证方式: %s", authType)
}

// parseResponse 解析响应数据
func (c *HTTPClient) parseResponse(body []byte, config model.JSON) ([]map[string]interface{}, error) {
	if items, err := parseArrayBody(body); err == nil {
		return items, nil
	}

	objResult, err := parseObjectBody(body)
	if err != nil {
		return nil, err
	}

	if items := extractPathArray(objResult, config); items != nil {
		return items, nil
	}
	if items := extractNamedArray(objResult, "data"); items != nil {
		return items, nil
	}
	if items := extractNamedArray(objResult, "items"); items != nil {
		return items, nil
	}
	return nil, fmt.Errorf("无法从响应中提取数据数组")
}

func parseArrayBody(body []byte) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func parseObjectBody(body []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return result, nil
}

func extractPathArray(objResult map[string]interface{}, config model.JSON) []map[string]interface{} {
	dataPath, ok := config["response_data_path"].(string)
	if !ok || dataPath == "" {
		return nil
	}

	current := objResult
	parts := strings.Split(dataPath, ".")
	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return nil
		}
		if i == len(parts)-1 {
			if arr, ok := val.([]interface{}); ok {
				return mapInterfaceArray(arr)
			}
			return nil
		}

		obj, ok := val.(map[string]interface{})
		if !ok {
			return nil
		}
		current = obj
	}
	return nil
}

func extractNamedArray(objResult map[string]interface{}, key string) []map[string]interface{} {
	arr, ok := objResult[key].([]interface{})
	if !ok {
		return nil
	}
	return mapInterfaceArray(arr)
}

func mapInterfaceArray(values []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]interface{}); ok {
			result = append(result, item)
		}
	}
	return result
}
