package googleapi

import (
	"context"

	"google.golang.org/api/forms/v1"
	"google.golang.org/api/option"
)

func InitGForm(ctx context.Context) (*forms.Service, error) {
	formsSvc, err := forms.NewService(ctx, option.WithCredentials(creds))
	return formsSvc, err
}
