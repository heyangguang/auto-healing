package healing

import (
	"fmt"

	"github.com/expr-lang/expr"
)

// ExpressionEvaluator 表达式求值器
type ExpressionEvaluator struct{}

// NewExpressionEvaluator 创建表达式求值器
func NewExpressionEvaluator() *ExpressionEvaluator {
	return &ExpressionEvaluator{}
}

// customFunctions 返回自定义函数列表。
// 注意：当函数参数类型不匹配时，使用 panic 抛出错误信息。
// expr 引擎会捕获 panic 并将其转换为执行错误，返回给用户。
func (e *ExpressionEvaluator) customFunctions() map[string]interface{} {
	functions := make(map[string]interface{})
	mergeExpressionFunctions(
		functions,
		sequenceFunctions(),
		collectionFunctions(),
		conversionFunctions(),
		stringFunctions(),
		mathFunctions(),
	)
	return functions
}

func mergeExpressionFunctions(dst map[string]interface{}, groups ...map[string]interface{}) {
	for _, group := range groups {
		for name, fn := range group {
			dst[name] = fn
		}
	}
}

// Evaluate 执行表达式并返回结果。
// expression: 表达式字符串，如 "validated_hosts[0].ip_address" 或 "join(hosts, ',')"
// context: 上下文数据，包含 incident、hosts、validated_hosts 等
func (e *ExpressionEvaluator) Evaluate(expression string, context map[string]interface{}) (interface{}, error) {
	if expression == "" {
		return nil, fmt.Errorf("表达式为空")
	}

	env := e.buildEnv(context)
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("编译表达式失败: %w", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("执行表达式失败: %w", err)
	}
	return result, nil
}

func (e *ExpressionEvaluator) buildEnv(context map[string]interface{}) map[string]interface{} {
	env := make(map[string]interface{}, len(context)+len(e.customFunctions()))
	for key, value := range context {
		env[key] = value
	}
	for key, value := range e.customFunctions() {
		env[key] = value
	}
	return env
}

// EvaluateToString 执行表达式并返回字符串结果
func (e *ExpressionEvaluator) EvaluateToString(expression string, context map[string]interface{}) (string, error) {
	result, err := e.Evaluate(expression, context)
	if err != nil {
		return "", err
	}
	return toStringValue(result), nil
}

// EvaluateMultiple 批量执行多个表达式。
// expressions: map[output_key] = expression
// 返回: map[output_key] = result
func (e *ExpressionEvaluator) EvaluateMultiple(expressions map[string]string, context map[string]interface{}) (map[string]interface{}, []error) {
	results := make(map[string]interface{}, len(expressions))
	var errors []error

	for outputKey, expression := range expressions {
		result, err := e.Evaluate(expression, context)
		if err != nil {
			errors = append(errors, fmt.Errorf("计算 %s 失败: %w", outputKey, err))
			continue
		}
		results[outputKey] = result
	}
	return results, errors
}
