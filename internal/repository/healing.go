package repository

import "errors"

var (
	ErrHealingFlowNotFound       = errors.New("自愈流程不存在")
	ErrHealingRuleNotFound       = errors.New("自愈规则不存在")
	ErrFlowInstanceNotFound      = errors.New("流程实例不存在")
	ErrApprovalTaskNotFound      = errors.New("审批任务不存在")
	ErrApprovalTaskNotPending    = errors.New("审批任务已处理")
	ErrFlowInstanceStateConflict = errors.New("流程实例状态已变更")
)
