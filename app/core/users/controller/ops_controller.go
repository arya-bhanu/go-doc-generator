package controller

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	docrepo "github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	"github.com/arya-bhanu/go-doc-generator/app/core/users/service"
	"github.com/arya-bhanu/go-doc-generator/constants"
	httpresponsewrapper "github.com/arya-bhanu/go-doc-generator/utils/http_response_wrapper"
)

func (h *UserOpsHandler) SubmitForm(c *gin.Context) {
	// get the user id from context
	var userID int
	userCtx, exist := c.Get(constants.UserOpsContextKey)
	if exist {
		userID = userCtx.(users.UserOps).ID
	}

	// user will submit with payload structure array of users.OpsUserDataField ([users.OpsUserDataField])
	var fields []users.OpsUserDataField
	if err := c.ShouldBindJSON(&fields); err != nil {
		c.JSON(http.StatusBadRequest, httpresponsewrapper.HttpResponse{
			Success: false,
			Err:     err.Error(),
			Msg:     "invalid request body",
			Data:    nil,
		})
		return
	}

	// convert that into a map object, map[OpsUserDataField.Variable] = OpsUserDataField
	// store and insert into ops_user.form_filled
	if err := service.SubmitOpsForm(userID, fields); err != nil {
		c.JSON(http.StatusInternalServerError, httpresponsewrapper.HttpResponse{
			Success: false,
			Err:     err.Error(),
			Msg:     "failed to save form",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{
		Success: true,
		Err:     "",
		Msg:     "form submitted successfully",
		Data:    nil,
	})
}
func (h *UserOpsHandler) OnLoginOps(c *gin.Context) {
	var userID int
	userCtx, exist := c.Get(constants.UserOpsContextKey)
	if exist {
		userID = userCtx.(users.UserOps).ID
	}

	userVariables := service.FetchOpsField()

	defaultField := service.CleanOpsField(userVariables)

	existingField := service.FetchExistingAnsweredOpsFieldForm(userID)

	// Merge previously answered values into the default field map.
	for key := range defaultField {
		if existing, ok := existingField[key]; ok {
			defaultField[key] = existing
		}
	}

	c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{
		Success: true,
		Err:     "",
		Msg:     "success fetch ops field",
		Data:    defaultField,
	})
}

// DeleteSession handles DELETE /api/ops/session.
// It permanently removes the form_sessions row for the authenticated user.
func (h *UserOpsHandler) DeleteSession(c *gin.Context) {
	var userID int
	userCtx, exist := c.Get(constants.UserOpsContextKey)
	if exist {
		userID = userCtx.(users.UserOps).ID
	}

	if err := docrepo.DeleteFormSessionByUserID(userID); err != nil {
		slog.Error("deleteSession: failed", "user_id", userID, "err", err.Error())
		c.JSON(http.StatusInternalServerError, httpresponsewrapper.HttpResponse{
			Success: false,
			Err:     err.Error(),
			Msg:     "failed to delete session",
			Data:    nil,
		})
		return
	}

	c.JSON(http.StatusOK, httpresponsewrapper.HttpResponse{
		Success: true,
		Err:     "",
		Msg:     "session deleted successfully",
		Data:    nil,
	})
}
