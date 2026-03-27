package middleware

import (
	"github.com/company/auto-healing/internal/database"
	"gorm.io/gorm"
)

func defaultDatabase() *gorm.DB {
	return database.DB
}
