package documents

import "time"

type DocumentVariable struct {
	ID              int       `json:"id"`
	Variable        string    `json:"variable"`
	Label           string    `json:"label"`
	CreatedAt       time.Time `json:"created_at"`
	Description     string    `json:"description"`
	Type            string    `json:"type,omitempty"`
	IsFillCustomers bool      `json:"is_fill_customers"`
}
