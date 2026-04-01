package users

import "time"

type UserOps struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type OpsUserDataField struct {
	Label    string `json:"label"`
	Variable string `json:"variable"`
	Answer   string `json:"answer"`
}

type OpsUserTable struct {
	ID         int                         `json:"id"`
	CreatedAt  time.Time                   `json:"created_at"`
	Name       string                      `json:"name"`
	Email      string                      `json:"email"`
	Role       string                      `json:"role"`
	UID        string                      `json:"uid"`
	FormFilled map[string]OpsUserDataField `json:"form_filled"`
}

type DocumentVariablesTable struct {
	ID              int    `json:"id"`
	Variable        string `json:"variable"`
	Label           string `json:"label"`
	Description     string `json:"description"`
	Type            string `json:"type"`
	IsFillCustomers bool   `json:"is_fill_customers"`
	PrefilledValue  string `json:"prefilled_value"`
}
