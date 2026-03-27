package service

import "github.com/company/auto-healing/internal/modules/ops/model"

func init() {
	AllDictionarySeeds = append(AllDictionarySeeds, opsContractDictionarySeeds()...)
}

func opsContractDictionarySeeds() []model.Dictionary {
	return []model.Dictionary{
		d("command_blacklist_match_type", "contains", "包含匹配", "Contains", "#1890ff", "blue", "", "", "", 0),
		d("command_blacklist_match_type", "regex", "正则匹配", "Regex", "#722ed1", "purple", "", "", "", 1),
		d("command_blacklist_match_type", "exact", "精确匹配", "Exact", "#52c41a", "green", "", "", "", 2),

		d("command_blacklist_severity", "critical", "极高", "Critical", "#f5222d", "red", "error", "", "", 0),
		d("command_blacklist_severity", "high", "高危", "High", "#fa8c16", "orange", "warning", "", "", 1),
		d("command_blacklist_severity", "medium", "中危", "Medium", "#1890ff", "blue", "processing", "", "", 2),

		d("command_blacklist_category", "filesystem", "文件系统", "Filesystem", "#1890ff", "blue", "", "", "", 0),
		d("command_blacklist_category", "network", "网络", "Network", "#13c2c2", "cyan", "", "", "", 1),
		d("command_blacklist_category", "system", "系统", "System", "#722ed1", "purple", "", "", "", 2),
		d("command_blacklist_category", "database", "数据库", "Database", "#52c41a", "green", "", "", "", 3),

		d("blacklist_exemption_status", "pending", "待审批", "Pending", "#fa8c16", "orange", "warning", "", "", 0),
		d("blacklist_exemption_status", "approved", "已批准", "Approved", "#52c41a", "green", "success", "", "", 1),
		d("blacklist_exemption_status", "rejected", "已拒绝", "Rejected", "#f5222d", "red", "error", "", "", 2),
		d("blacklist_exemption_status", "expired", "已过期", "Expired", "#8c8c8c", "default", "default", "", "", 3),
	}
}
