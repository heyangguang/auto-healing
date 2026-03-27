package audit

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

func countWithClone(db *gorm.DB) (int64, error) {
	var count int64
	return count, db.Session(&gorm.Session{}).Count(&count).Error
}

func auditCount(db *gorm.DB) (int64, error) {
	var count int64
	return count, db.Count(&count).Error
}

func orderClause(sortBy, sortOrder string, allowed map[string]bool) string {
	field := "created_at"
	if allowed[sortBy] {
		field = sortBy
	}
	order := "DESC"
	if sortOrder == "asc" || sortOrder == "ASC" {
		order = "ASC"
	}
	return fmt.Sprintf("%s %s", field, order)
}

func applyDaysFilter(db *gorm.DB, days int) *gorm.DB {
	if days <= 0 {
		return db
	}
	return db.Where("created_at >= ?", time.Now().AddDate(0, 0, -days))
}
