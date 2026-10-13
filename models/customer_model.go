package models

type Customer struct {
	ID    string `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name  string `gorm:"not null" json:"name"`
	Email string `gorm:"uniqueIndex;not null" json:"email"`
}

type CustomerUpdate struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}
