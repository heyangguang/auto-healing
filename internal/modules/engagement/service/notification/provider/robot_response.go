package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func buildRobotSendResponse(
	resp *http.Response,
	providerName string,
) (*SendResponse, error) {
	body, _ := io.ReadAll(resp.Body)
	result, err := parseRobotResponse(resp.StatusCode, body, providerName)
	if err != nil {
		return &SendResponse{Success: false, ErrorMessage: err.Error()}, err
	}
	return &SendResponse{
		Success:      true,
		ResponseData: result,
	}, nil
}

func validateRobotResponse(resp *http.Response, providerName string) error {
	body, _ := io.ReadAll(resp.Body)
	_, err := parseRobotResponse(resp.StatusCode, body, providerName)
	if err != nil {
		return fmt.Errorf("测试失败: %w", err)
	}
	return nil
}

func parseRobotResponse(
	statusCode int,
	body []byte,
	providerName string,
) (map[string]interface{}, error) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s返回错误状态码: %d, body: %s", providerName, statusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("%s响应解析失败: %w", providerName, err)
	}

	errcodeValue, ok := result["errcode"]
	if !ok {
		return nil, fmt.Errorf("%s响应缺少 errcode: %s", providerName, string(body))
	}
	errcode, ok := errcodeValue.(float64)
	if !ok {
		return nil, fmt.Errorf("%s响应 errcode 类型无效: %T", providerName, errcodeValue)
	}
	if errcode == 0 {
		return result, nil
	}

	errmsg, _ := result["errmsg"].(string)
	if errmsg == "" {
		errmsg = "unknown error"
	}
	return result, fmt.Errorf("%s发送失败: %s (errcode: %.0f)", providerName, errmsg, errcode)
}
