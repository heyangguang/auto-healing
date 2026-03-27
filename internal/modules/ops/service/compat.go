package service

import "github.com/company/auto-healing/internal/database"

func NewDictionaryService() *DictionaryService {
	return NewDictionaryServiceWithDB(database.DB)
}

func NewCommandBlacklistService() *CommandBlacklistService {
	return NewCommandBlacklistServiceWithDB(database.DB)
}

func NewBlacklistExemptionService() *BlacklistExemptionService {
	return NewBlacklistExemptionServiceWithDB(database.DB)
}
