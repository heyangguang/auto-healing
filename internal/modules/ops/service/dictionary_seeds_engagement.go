package service

import "github.com/company/auto-healing/internal/modules/ops/model"

func init() {
	AllDictionarySeeds = append(AllDictionarySeeds, engagementDictionarySeeds()...)
}

func engagementDictionarySeeds() []model.Dictionary {
	return []model.Dictionary{
		d("execution_triggered_by", "manual", "手动触发", "Manual", "#fa8c16", "orange", "", "", "", 0),
		d("execution_triggered_by", "scheduler:cron", "定时触发(Cron)", "Scheduler Cron", "#1890ff", "blue", "", "", "", 1),
		d("execution_triggered_by", "scheduler:once", "定时触发(单次)", "Scheduler Once", "#13c2c2", "cyan", "", "", "", 2),
		d("execution_triggered_by", "healing", "自愈触发", "Healing", "#52c41a", "green", "", "", "", 3),
	}
}
