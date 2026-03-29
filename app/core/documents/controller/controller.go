package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	docsvc "github.com/arya-bhanu/go-doc-generator/app/core/documents/service"
)

// Handler holds the service dependencies for document controllers.
type Handler struct {
	DocService *docsvc.DocumentService
}

// NewHandler constructs a Handler with the given DocumentService.
func NewHandler(docService *docsvc.DocumentService) *Handler {
	return &Handler{DocService: docService}
}

// CreateGoogleFormController binds the request payload, calls ProcessDocuments
// via the injected DocumentService, and returns the fetched document files.
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
