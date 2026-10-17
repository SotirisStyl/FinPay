package models

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

type LoanStatus string

const (
	LoanStatusPending  LoanStatus = "PENDING"
	LoanStatusApproved LoanStatus = "APPROVED"
	LoanStatusRejected LoanStatus = "REJECTED"
	LoanStatusActive   LoanStatus = "ACTIVE"
	LoanStatusPaidOff  LoanStatus = "PAID_OFF"
)

type Loan struct {
	ID         string          `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	CustomerID string          `gorm:"type:uuid;not null" json:"customer_id"`
	Amount     decimal.Decimal `gorm:"not null" json:"amount"`
	Status     string          `gorm:"not null" json:"status"`
	Customer   Customer        `gorm:"foreignKey:CustomerID;references:ID" json:"-"`
}

type CreateLoanRequest struct {
	Amount *decimal.Decimal `json:"amount"`
	Status json.RawMessage  `json:"status"`
}
