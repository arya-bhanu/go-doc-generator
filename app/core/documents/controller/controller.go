package controller

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/arya-bhanu/go-doc-generator/constants"
)

func CreateGoogleFormController(c *gin.Context) {
	var payload CreateFormPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		slog.Warn("wrong json format or empty form_ids", "err", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "wrong json format or empty form_ids"})
		return
	}

	fmt.Println(c.Get(constants.UserEmailContextKey))

}
