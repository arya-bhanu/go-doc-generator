package server

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/arya-bhanu/go-doc-generator/constants"
	httpresponsewrapper "github.com/arya-bhanu/go-doc-generator/utils/http_response_wrapper"
)

// AuthMiddleware validates:
//  1. X-App-Identity: Basic <base64("username:password")>
//  2. Authorization: Bearer <supabase-jwt>  (verified via JWK)
//
// On success it stores the authenticated user's email under UserEmailContextKey.
func AuthMiddleware(keySet jwk.Set, issuer string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── 1. Basic Auth via X-App-Identity header ──────────────────────────
		appIdentity := c.GetHeader("X-App-Identity")
		if appIdentity == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing X-App-Identity header",
			})
			return
		}

		// Expected format: "Basic <base64(username:password)>"
		parts := strings.SplitN(appIdentity, " ", 2)
		if len(parts) != 2 || parts[0] != "Basic" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid X-App-Identity format, expected: Basic <base64>",
			})
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid base64 encoding in X-App-Identity",
			})
			return
		}

		// Decoded format: "username:password"
		creds := strings.SplitN(string(decoded), ":", 2)
		if len(creds) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid credentials format in X-App-Identity",
			})
			return
		}

		slog.Info("creds", "user", creds[0], "pass", creds[1])

		expectedUsername := os.Getenv("BASIC_AUTH_USERNAME")
		expectedPassword := os.Getenv("BASIC_AUTH_PASSWORD")

		if creds[0] != expectedUsername || creds[1] != expectedPassword {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid basic auth credentials",
			})
			return
		}

		// ── 2. JWT verification via Authorization: Bearer header ──────────────
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing Authorization header",
			})
			return
		}

		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid Authorization format, expected: Bearer <token>",
			})
			return
		}

		rawToken := tokenParts[1]

		token, err := jwt.Parse(
			[]byte(rawToken),
			jwt.WithKeySet(keySet),
			jwt.WithValidate(true),
			jwt.WithAudience("authenticated"),
			jwt.WithIssuer(issuer),
		)

		if err != nil {
			slog.Error("invalid or expired JWT", "err", err.Error())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired JWT",
			})
			return
		}

		// ── 3. Extract email claim & store in context ─────────────────────────
		emailVal, ok := token.Get("email")
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "failed to extract email claim from JWT",
			})
			return
		}

		emailStr, ok := emailVal.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "email claim is not a string",
			})
			return
		}

		c.Set(constants.UserEmailContextKey, emailStr)
		c.Next()
	}
}

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			c.JSON(http.StatusInternalServerError, httpresponsewrapper.HttpResponseFunc(httpresponsewrapper.HttpResponse{
				Err:     err.Error(),
				Msg:     "",
				Success: false,
			}))
		}
	}
}
