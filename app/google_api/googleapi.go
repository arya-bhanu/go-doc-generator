package googleapi

import (
	"context"
	"log/slog"
	"os"

	"golang.org/x/oauth2/google"
)

var (
	creds *google.Credentials
)

func Init(ctx context.Context) error {
	jsonKey, err := os.ReadFile("service-account.json")
	if err != nil {
		slog.Error(err.Error())
		return err
	}
	creds, err = google.CredentialsFromJSONWithType(ctx, jsonKey, google.ServiceAccount,
		"https://www.googleapis.com/auth/drive",

		// Forms - buat, edit, dan baca respons
		"https://www.googleapis.com/auth/forms.body",
		"https://www.googleapis.com/auth/forms.responses.readonly",

		// Docs - manipulasi dokumen
		"https://www.googleapis.com/auth/documents",
	)

	return err
}
