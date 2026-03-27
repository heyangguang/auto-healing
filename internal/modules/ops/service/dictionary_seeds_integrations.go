package service

import "github.com/company/auto-healing/internal/modules/ops/model"

func init() {
	AllDictionarySeeds = append(AllDictionarySeeds, integrationDictionarySeeds()...)
}

func integrationDictionarySeeds() []model.Dictionary {
	return []model.Dictionary{
		d("secrets_source_status", "active", "启用", "Active", "#52c41a", "green", "success", "", "", 0),
		d("secrets_source_status", "inactive", "停用", "Inactive", "#d9d9d9", "default", "default", "", "", 1),
	}
}
