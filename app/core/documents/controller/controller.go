package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	docsvc "github.com/arya-bhanu/go-doc-generator/app/core/documents/service"
)

type Handler struct {
	DocService *docsvc.DocumentService
}

func NewHandler(docService *docsvc.DocumentService) *Handler {
	return &Handler{DocService: docService}
}

func (h *Handler) CreateGoogleFormController(c *gin.Context) {
	var payload CreateFormPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		slog.Warn("wrong json format or empty form_ids", "err", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong json format or empty form_ids"})
		return
	}

	docTitles, err := h.DocService.ProcessDocuments(c, payload.DocIDS)
	if err != nil {
		slog.Error("failed to process documents", "err", err.Error())
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"document_titles": docTitles})
}
