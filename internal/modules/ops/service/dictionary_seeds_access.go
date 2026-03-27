package service

import "github.com/company/auto-healing/internal/modules/ops/model"

func init() {
	AllDictionarySeeds = append(AllDictionarySeeds, accessDictionarySeeds()...)
}

func accessDictionarySeeds() []model.Dictionary {
	return []model.Dictionary{
		d("invitation_status", "pending", "待接受", "Pending", "#fa8c16", "orange", "warning", "", "", 0),
		d("invitation_status", "accepted", "已接受", "Accepted", "#52c41a", "green", "success", "", "", 1),
		d("invitation_status", "expired", "已过期", "Expired", "#8c8c8c", "default", "default", "", "", 2),
		d("invitation_status", "cancelled", "已取消", "Cancelled", "#8c8c8c", "default", "default", "", "", 3),

		d("impersonation_status", "pending", "待审批", "Pending", "#fa8c16", "orange", "warning", "", "", 0),
		d("impersonation_status", "approved", "已批准", "Approved", "#52c41a", "green", "success", "", "", 1),
		d("impersonation_status", "rejected", "已拒绝", "Rejected", "#f5222d", "red", "error", "", "", 2),
		d("impersonation_status", "active", "会话中", "Active", "#1890ff", "blue", "processing", "", "", 3),
		d("impersonation_status", "completed", "已完成", "Completed", "#52c41a", "green", "success", "", "", 4),
		d("impersonation_status", "expired", "已过期", "Expired", "#8c8c8c", "default", "default", "", "", 5),
		d("impersonation_status", "cancelled", "已撤销", "Cancelled", "#8c8c8c", "default", "default", "", "", 6),
	}
}
