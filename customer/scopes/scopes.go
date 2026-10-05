package scopes

import "gorm.io/gorm"

// ById filters records by id
func ByID(id string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("customers.id = ?", id)
	}
}
