package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	docsvc "github.com/arya-bhanu/go-doc-generator/app/core/documents/service"
	formservice "github.com/arya-bhanu/go-doc-generator/app/core/form/service"
	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	"github.com/arya-bhanu/go-doc-generator/constants"
	formconst "github.com/arya-bhanu/go-doc-generator/constants/form_const"
	httpresponsewrapper "github.com/arya-bhanu/go-doc-generator/utils/http_response_wrapper"
)

type Handler struct {
	DocService  *docsvc.DocumentService
	FormService *formservice.FormService
}

func NewHandler(docService *docsvc.DocumentService, formService *formservice.FormService) *Handler {
	return &Handler{DocService: docService, FormService: formService}
}

// RefreshDocumentTemplates handles POST /api/document/refresh.
// It fetches all .docx files from the Google Drive template folder
// (GOOGLE_DRIVE_TEMPLATE_ID) and upserts them into stored_document_templates.
func (h *Handler) RefreshDocumentTemplates(c *gin.Context) {
	templates, err := h.DocService.RefreshDocumentTemplates()
	if err != nil {
		slog.Error("refreshDocumentTemplates: failed", "err", err.Error())
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{
		Success: true,
		Err:     "",
		Msg:     "document templates refreshed successfully",
		Data:    templates,
	})
}

func (h *Handler) CreateGoogleFormController(c *gin.Context) {
	var payload CreateFormPayload
	var userID int
	user, ex := c.Get(constants.UserOpsContextKey)
	if ex {
		userID = user.(users.UserOps).ID
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		slog.Warn("wrong json format or empty form_ids", "err", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong json format or empty form_ids"})
		return
	}

	userVars, answeredQuestCust, varPayload, err := h.DocService.ProcessDocuments(c, payload.DocIDS)
	if err != nil {
		slog.Error("failed to process documents", "err", err.Error())
		c.Error(err)
		return
	}

	// If all customer variables are already answered (userVars is empty),
	// skip form generation and send the filled documents directly via email.
	// Ops variable mapping ({curly-brace} substitution) is handled inside
	// SendDocumentsDirect using payload.UserID.
	if len(userVars) == 0 {
		if err = h.DocService.SendDocumentsDirect(varPayload, answeredQuestCust); err != nil {
			slog.Error("failed to send documents directly", "err", err.Error())
			c.Error(err)
			return
		}

		go func() {
			if err := h.DocService.ClearFormScaffoldCust(userID); err != nil {
				slog.Error("failed to clear form_scaffold_cust",
					"userID", userID, "err", err.Error())
			}
		}()

		c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{
			Success: true,
			Err:     "",
			Msg:     "all variables already answered — document sent to email",
			Data:    "",
		})
		return
	}

	// GENERATE GOOGLE FORM IF ONLY userVars is not an empty map
	// this will generate a google form using google form API service
	formRes, err := h.FormService.GenerateGoogleForm(c.Request.Context(), formconst.FormCustTitle, userVars)
	if err != nil {
		slog.Error("failed to generate google form", "err", err.Error())
		c.Error(err)
		return
	}

	// will set the formLink into the payload
	varPayload.FormLink = &formRes.FormLink
	varPayload.FormID = &formRes.FormID

	// If a form_session already exists for this user (matched by user_id),
	// update form_link, form_scaffold_cust, doc_details, and form_id only.
	// Otherwise create a new session row.
	if err = h.DocService.UpsertSession(varPayload); err != nil {
		slog.Error("failed to upsert session", "err", err.Error())
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{Success: true, Err: "", Msg: "success create google form", Data: formRes.FormLink})
}
