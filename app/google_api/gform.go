package googleapi

import (
	"context"
	"net/http"

	"google.golang.org/api/forms/v1"
	"google.golang.org/api/option"
)

func InitGForm(ctx context.Context) (*forms.Service, error) {
	formsSvc, err := forms.NewService(ctx, option.WithCredentials(creds))
	return formsSvc, err
}

// InitGFormOAuth creates a Forms service backed by an OAuth2 HTTP client.
// The client is obtained from InitOAuthHTTPClient.
func InitGFormOAuth(ctx context.Context, httpClient *http.Client) (*forms.Service, error) {
	return forms.NewService(ctx, option.WithHTTPClient(httpClient))
}
