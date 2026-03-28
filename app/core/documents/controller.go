package documents

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
)

func CreateGoogleFormController(ctx *gin.Context) {
	var form_ids []string
	raw_form_ids := ctx.PostForm("form_ids")
	err := json.Unmarshal([]byte(raw_form_ids), &form_ids)
	if err != nil {
		slog.Error("[CreateGoogleFormController] error unmarshal raw_form_ids into form_ids", "err", err.Error())
		ctx.Error(errors.New("error unmarshal raw_form_ids into form_ids"))
	}
}
