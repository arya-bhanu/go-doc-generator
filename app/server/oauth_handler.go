package server

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	googleapi "github.com/arya-bhanu/go-doc-generator/app/google_api"
)

const (
	oauthTokenFile    = "token.json"
	oauthState        = "state-token"
	oauthTemplatePath = "template/index.html"
)

// callbackPageData is the data passed to template/index.html.
type callbackPageData struct {
	Success bool
	Title   string
	Detail  template.HTML // pre-validated HTML, not escaped
}

// oauthCallbackURL returns the redirect URL that Google will use after the
// user grants (or denies) access.  It is derived from APP_BASE_URL when set,
// otherwise constructed from APP_PORT (default 8080).
func oauthCallbackURL() string {
	if base := os.Getenv("APP_BASE_URL"); base != "" {
		return base + "/api/admin/oauth/callback"
	}
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	return "http://localhost:" + port + "/api/admin/oauth/callback"
}

// oauthAuthenticateHandler handles GET /api/admin/oauth/authenticate.
// It returns a JSON response containing the Google consent URL.
// Open the returned auth_url in a browser to start the OAuth flow;
// after granting access Google will redirect to /api/admin/oauth/callback
// which saves token.json automatically.
func oauthAuthenticateHandler(c *gin.Context) {
	config, err := googleapi.GetOAuthConfig()
	if err != nil {
		slog.Error("oauth authenticate: load config", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load OAuth config: " + err.Error()})
		return
	}

	config.RedirectURL = oauthCallbackURL()

	authURL := config.AuthCodeURL(oauthState, oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	slog.Info("oauth authenticate: auth URL generated")

	c.JSON(http.StatusOK, gin.H{
		"message":  "Open the auth_url in your browser to authenticate with Google. After granting access, token.json will be generated automatically.",
		"auth_url": authURL,
	})
}

// callbackPage parses template/index.html and writes the rendered HTML to the response.
func callbackPage(c *gin.Context, status int, success bool, title, detail string) {
	tmpl, err := template.ParseFiles(oauthTemplatePath)
	if err != nil {
		slog.Error("callbackPage: parse template", "error", err)
		c.String(http.StatusInternalServerError, "failed to load page template: %v", err)
		return
	}

	data := callbackPageData{
		Success: success,
		Title:   title,
		Detail:  template.HTML(detail), //nolint:gosec // detail is constructed internally, not from user input
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error("callbackPage: execute template", "error", err)
		c.String(http.StatusInternalServerError, "failed to render page: %v", err)
		return
	}

	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

// oauthCallbackHandler handles GET /api/admin/oauth/callback.
// Google redirects the browser here after the user grants access.
// The handler exchanges the one-time code for a token and saves it to token.json.
func oauthCallbackHandler(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		slog.Warn("oauth callback: missing authorization code")
		callbackPage(c, http.StatusBadRequest, false,
			"Authorization Failed",
			"No authorization code was received. Please go back and try authenticating again.",
		)
		return
	}

	config, err := googleapi.GetOAuthConfig()
	if err != nil {
		slog.Error("oauth callback: load config", "error", err)
		callbackPage(c, http.StatusInternalServerError, false,
			"Configuration Error",
			"Failed to load the OAuth configuration: <code>"+err.Error()+"</code>",
		)
		return
	}

	config.RedirectURL = oauthCallbackURL()

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		slog.Error("oauth callback: exchange code", "error", err)
		callbackPage(c, http.StatusInternalServerError, false,
			"Token Exchange Failed",
			"Could not exchange the authorization code for a token: <code>"+err.Error()+"</code>",
		)
		return
	}

	f, err := os.Create(oauthTokenFile)
	if err != nil {
		slog.Error("oauth callback: create token file", "error", err)
		callbackPage(c, http.StatusInternalServerError, false,
			"File Error",
			"Authentication succeeded but <code>token.json</code> could not be created: <code>"+err.Error()+"</code>",
		)
		return
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		slog.Error("oauth callback: write token", "error", err)
		callbackPage(c, http.StatusInternalServerError, false,
			"File Error",
			"Authentication succeeded but failed to write <code>token.json</code>: <code>"+err.Error()+"</code>",
		)
		return
	}

	slog.Info("oauth callback: token saved", "file", oauthTokenFile)
	callbackPage(c, http.StatusOK, true,
		"Authentication Successful",
		"You have successfully authenticated with Google. Your credentials have been saved to <code>token.json</code> and the server can now access Drive &amp; Forms on your behalf.",
	)
}
