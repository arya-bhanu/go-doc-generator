package googleapi

import (
	"context"
	"net/http"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func InitGDrive(ctx context.Context) (*drive.Service, error) {
	driveSvc, err := drive.NewService(ctx, option.WithCredentials(creds))
	return driveSvc, err
}

// InitGDriveOAuth creates a Drive service backed by an OAuth2 HTTP client.
// The client is obtained from InitOAuthHTTPClient.
func InitGDriveOAuth(ctx context.Context, httpClient *http.Client) (*drive.Service, error) {
	return drive.NewService(ctx, option.WithHTTPClient(httpClient))
}
