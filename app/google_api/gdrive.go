package googleapi

import (
	"context"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func InitGDrive(ctx context.Context) (*drive.Service, error) {
	driveSvc, err := drive.NewService(ctx, option.WithCredentials(creds))
	return driveSvc, err
}
