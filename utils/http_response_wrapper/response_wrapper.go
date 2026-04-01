package httpresponsewrapper

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

type HttpResponse struct {
	Err     string `json:"err"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
	Data    any    `json:"data"`
}

func HttpResponseFunc(res HttpResponse) gin.H {
	// 1. Ubah struct jadi []byte (JSON)
	jsonData, err := json.Marshal(res)
	if err != nil {
		return gin.H{"error": "failed to marshal"}
	}

	// 2. Ubah []byte jadi map
	var response map[string]any
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return gin.H{"error": "failed to unmarshal"}
	}

	return gin.H(response)
}
