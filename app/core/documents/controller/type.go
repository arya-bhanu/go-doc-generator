package controller

type CreateFormPayload struct {
	FormIDS []string `json:"form_ids" binding:"required"`
}
