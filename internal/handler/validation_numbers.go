package handler

import "fmt"

func validateNonNegativeMaxFailures(maxFailures *int) error {
	if maxFailures != nil && *maxFailures < 0 {
		return fmt.Errorf("最大连续失败次数不能为负数")
	}
	return nil
}
