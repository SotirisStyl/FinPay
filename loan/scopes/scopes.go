package scopes

import "gorm.io/gorm"

func ByCustomerID(customerID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("customers.id = ? AND customers.deleted_at IS NULL", customerID)
	}
}
