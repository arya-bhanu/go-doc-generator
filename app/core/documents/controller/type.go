package controller

type CreateFormPayload struct {
	DocIDS []string `json:"doc_ids" binding:"required"`
}
