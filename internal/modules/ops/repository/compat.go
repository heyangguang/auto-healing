package repository

import "github.com/company/auto-healing/internal/database"

func NewDictionaryRepository() *DictionaryRepository {
	return NewDictionaryRepositoryWithDB(database.DB)
}

func NewCommandBlacklistRepository() *CommandBlacklistRepository {
	return NewCommandBlacklistRepositoryWithDB(database.DB)
}
