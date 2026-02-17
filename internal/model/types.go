package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON 自定义JSON类型用于GORM
type JSON map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	case map[string]interface{}:
		*j = JSON(v)
		return nil
	default:
		// 尝试 JSON 序列化后反序列化
		data, err := json.Marshal(v)
		if err != nil {
			return errors.New("unsupported type for JSON scan")
		}
		return json.Unmarshal(data, j)
	}
}

// JSONArray 自定义JSON数组类型
type JSONArray []interface{}

// Value 实现 driver.Valuer 接口
func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	case []interface{}:
		*j = JSONArray(v)
		return nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return errors.New("unsupported type for JSONArray scan")
		}
		return json.Unmarshal(data, j)
	}
}

// StringArray 字符串数组类型
type StringArray []string

// Value 实现 driver.Valuer 接口
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// Scan 实现 sql.Scanner 接口
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return errors.New("unsupported type for StringArray scan")
		}
		return json.Unmarshal(data, s)
	}
}
