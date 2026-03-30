package documents

import "time"

type DocumentVariable struct {
	ID              int       `json:"id"`
	Variable        string    `json:"variable"`
	Label           string    `json:"label"`
	CreatedAt       time.Time `json:"created_at"`
	Description     *string   `json:"description"`
	Type            *string   `json:"type"`
	IsFillCustomers bool      `json:"is_fill_customers"`
}

type DocumentDetail struct {
	DocTempTitle  string `json:"doc_temp_title"`
	DocID         string `json:"doc_id"`
	OriginalTitle string `json:"original_title"`
}

type FormSessions struct {
	DocDetails       []DocumentDetail             `json:"doc_details"`
	FormLink         string                       `json:"form_link,omitempty"`
	FormScaffoldCust map[string]*DocumentVariable `json:"form_scaffold_cust"`
	FormScaffoldOps  map[string]*DocumentVariable `json:"form_scaffold_ops"`
	UserID           int                          `json:"user_id"`
	FormID           string                       `json:"form_id"`
}
