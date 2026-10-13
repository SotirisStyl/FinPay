package models

type Loan struct {
	ID         string   `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CustomerID string   `gorm:"type:uuid;not null" json:"customer_id"`
	Amount     float64  `gorm:"not null" json:"amount"`
	Status     string   `gorm:"not null" json:"status"`
	Customer   Customer `gorm:"foreignKey:CustomerID;references:ID" json:"-"`
}
