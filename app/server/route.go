package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwk"

	ctr "github.com/arya-bhanu/go-doc-generator/app/core/documents/controller"
)

func registerRoutes(r *gin.Engine) {
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-App-Identity"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowOrigins:     []string{"*"},
	}))

	r.Use(ErrorMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// SUPABASE_ISSUER_URL is the JWKS endpoint used both for key fetching
	// and as the expected `iss` claim value in incoming JWTs.
	// Example: https://<project>.supabase.co/auth/v1/.well-known/jwks.json
	jwksURL := os.Getenv("SUPABASE_ISSUER_URL")
	issuer := jwksURL
	if jwksURL == "" {
		slog.Error("SUPABASE_ISSUER_URL env var is not set")
		os.Exit(1)
	}

	jwksURL = fmt.Sprintf("%s/.well-known/jwks.json", jwksURL)

	ctx := context.Background()

	// Create a JWK cache that auto-refreshes the remote key set.
	cache := jwk.NewCache(ctx)
	if err := cache.Register(jwksURL, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		slog.Error("failed to register JWKS URL with JWK cache", "error", err)
		os.Exit(1)
	}

	// Perform an initial fetch to verify the endpoint is reachable at startup.
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		slog.Error("failed to fetch JWK set from JWKS URL", "url", jwksURL, "error", err)
		os.Exit(1)
	}

	// CachedSet implements jwk.Set and is backed by the auto-refreshing cache.
	keySet := jwk.NewCachedSet(cache, jwksURL)

	// /api group – protected by auth middleware
	api := r.Group("/api")
	api.Use(AuthMiddleware(keySet, issuer))
	{
		customer := api.Group("/customer")
		{
			customer.POST("/create-form", ctr.CreateGoogleFormController)
		}
	}
}
