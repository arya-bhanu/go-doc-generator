package googleapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	creds *google.Credentials
)

var oauthScopes = []string{
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/drive.file",
	"https://www.googleapis.com/auth/forms.body",
	"https://www.googleapis.com/auth/forms.responses.readonly",
	"https://www.googleapis.com/auth/documents",
}

func Init(ctx context.Context) error {
	// Prefer the env-var path (production / Render) over the local file.
	var jsonKey []byte
	if raw := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"); raw != "" {
		slog.Info("googleapi: using GOOGLE_SERVICE_ACCOUNT_JSON env var for service account")
		jsonKey = []byte(raw)
	} else {
		var err error
		jsonKey, err = os.ReadFile("service-account.json")
		if err != nil {
			slog.Error(err.Error())
			return err
		}
	}

	var err error
	creds, err = google.CredentialsFromJSONWithType(ctx, jsonKey, google.ServiceAccount,
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/drive.file",

		"https://www.googleapis.com/auth/forms.body",
		"https://www.googleapis.com/auth/forms.responses.readonly",

		"https://www.googleapis.com/auth/documents",
	)

	return err
}

// GetOAuthConfig reads oauth-client-secret.json and returns a configured
// *oauth2.Config with the standard Drive + Forms scopes.
// The caller is responsible for setting RedirectURL before use.
func GetOAuthConfig() (*oauth2.Config, error) {
	var oauthData []byte
	if raw := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET_JSON"); raw != "" {
		slog.Info("googleapi: using GOOGLE_OAUTH_CLIENT_SECRET_JSON env var for OAuth config")
		oauthData = []byte(raw)
	} else {
		var err error
		oauthData, err = os.ReadFile("oauth-client-secret.json")
		if err != nil {
			return nil, fmt.Errorf("read oauth-client-secret.json: %w", err)
		}
	}

	config, err := google.ConfigFromJSON(oauthData, oauthScopes...)
	if err != nil {
		return nil, fmt.Errorf("parse oauth config: %w", err)
	}

	return config, nil
}

// InitOAuthHTTPClient loads oauth-client-secret.json + token.json and
// returns an *http.Client whose token is auto-refreshed — no consent
// screen after the first run of tools/get_token/.
func InitOAuthHTTPClient(ctx context.Context) (*http.Client, error) {
	var oauthData []byte
	if raw := os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET_JSON"); raw != "" {
		slog.Info("googleapi: using GOOGLE_OAUTH_CLIENT_SECRET_JSON env var for OAuth client")
		oauthData = []byte(raw)
	} else {
		var err error
		oauthData, err = os.ReadFile("oauth-client-secret.json")
		if err != nil {
			return nil, fmt.Errorf("read oauth-client-secret.json: %w", err)
		}
	}

	config, err := google.ConfigFromJSON(oauthData, oauthScopes...)
	if err != nil {
		return nil, fmt.Errorf("parse oauth config: %w", err)
	}
	config.RedirectURL = "http://localhost:9999/callback"

	var tokenData []byte
	if raw := os.Getenv("GOOGLE_OAUTH_TOKEN_JSON"); raw != "" {
		slog.Info("googleapi: using GOOGLE_OAUTH_TOKEN_JSON env var for OAuth token")
		tokenData = []byte(raw)
	} else {
		tokenData, err = os.ReadFile("token.json")
		if err != nil {
			return nil, fmt.Errorf("read token.json (run: go run ./tools/get_token/): %w", err)
		}
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return nil, fmt.Errorf("parse token.json: %w", err)
	}

	tokenSource := config.TokenSource(ctx, &token)
	return oauth2.NewClient(ctx, tokenSource), nil
}
