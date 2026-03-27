package repository

import "gorm.io/gorm"

func countWithSession(query *gorm.DB) (int64, error) {
	var total int64
	err := query.Session(&gorm.Session{}).Count(&total).Error
	return total, err
}
