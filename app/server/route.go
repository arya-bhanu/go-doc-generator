package server

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/arya-bhanu/go-doc-generator/app/core/documents"
)

func registerRoutes(r *gin.Engine) {
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowOrigins:     []string{"*"},
	}))

	r.Use(ErrorHandler())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	r.Group("/api", func(ctx *gin.Context) {
		r.Group("/customer", func(ctx *gin.Context) {
			r.POST("/create-form", documents.CreateGoogleFormController)
		})
	})
}
