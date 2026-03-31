package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	docsvc "github.com/arya-bhanu/go-doc-generator/app/core/documents/service"
	formservice "github.com/arya-bhanu/go-doc-generator/app/core/form/service"
	httpresponsewrapper "github.com/arya-bhanu/go-doc-generator/utils/http_response_wrapper"
)

type Handler struct {
	DocService  *docsvc.DocumentService
	FormService *formservice.FormService
}

func NewHandler(docService *docsvc.DocumentService, formService *formservice.FormService) *Handler {
	return &Handler{DocService: docService, FormService: formService}
}

func (h *Handler) CreateGoogleFormController(c *gin.Context) {
	var payload CreateFormPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		slog.Warn("wrong json format or empty form_ids", "err", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong json format or empty form_ids"})
		return
	}

	userVars, _, varPayload, err := h.DocService.ProcessDocuments(c, payload.DocIDS)
	if err != nil {
		slog.Error("failed to process documents", "err", err.Error())
		c.Error(err)
		return
	}

	formTitle := "Document Form"
	if len(varPayload.DocDetails) > 0 {
		formTitle = varPayload.DocDetails[0].DocTempTitle
	}

	// this will generate a google form using google form API service
	formLink, err := h.FormService.GenerateGoogleForm(c.Request.Context(), formTitle, userVars)
	if err != nil {
		slog.Error("failed to generate google form", "err", err.Error())
		c.Error(err)
		return
	}

	// will set the formLink into the payload
	varPayload.FormLink = formRes.FormLink
	varPayload.FormID = formRes.FormID

	// it will inserted into supabase
	if err = h.DocService.CreateSession(varPayload); err != nil {
		slog.Error("failed to create session", "err", err.Error())
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{Success: true, Err: "", Msg: "success create google form", Data: formRes.FormLink})
}
