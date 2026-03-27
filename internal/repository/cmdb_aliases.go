package repository

import (
	cmdbrepo "github.com/company/auto-healing/internal/platform/repository/cmdb"
	"gorm.io/gorm"
)

type CMDBItemRepository = cmdbrepo.CMDBItemRepository
type CMDBItemBasic = cmdbrepo.CMDBItemBasic

var ErrCMDBItemNotFound = cmdbrepo.ErrCMDBItemNotFound

func NewCMDBItemRepository() *CMDBItemRepository {
	return cmdbrepo.NewCMDBItemRepository()
}

func NewCMDBItemRepositoryWithDB(db *gorm.DB) *CMDBItemRepository {
	return cmdbrepo.NewCMDBItemRepositoryWithDB(db)
}
