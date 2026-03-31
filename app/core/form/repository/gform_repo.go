package repository

import (
	"context"
	"fmt"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/forms/v1"

	"github.com/arya-bhanu/go-doc-generator/app/core/form/service"
)

type GFormRepository interface {
	CreateForm(ctx context.Context, title string, items []*forms.Item) (service.GoogleFormRes, error)
}

type GFormRepo struct {
	formsSvc *forms.Service
	driveSvc *drive.Service
}

func NewGFormRepo(formsSvc *forms.Service, driveSvc *drive.Service) *GFormRepo {
	return &GFormRepo{formsSvc: formsSvc, driveSvc: driveSvc}
}

func (r *GFormRepo) CreateForm(ctx context.Context, title string, items []*forms.Item) (service.GoogleFormRes, error) {
	form, err := r.formsSvc.Forms.Create(&forms.Form{
		Info: &forms.Info{
			Title: title,
		},
	}).Context(ctx).Do()
	if err != nil {
		return service.GoogleFormRes{}, fmt.Errorf("gform: create form: %w", err)
	}

	if len(items) == 0 {
		return service.GoogleFormRes{FormLink: form.ResponderUri, FormID: form.FormId}, nil
	}

	requests := make([]*forms.Request, 0, len(items))
	for i, item := range items {
		loc := &forms.Location{Index: int64(i)}
		loc.ForceSendFields = []string{"Index"}

		requests = append(requests, &forms.Request{
			CreateItem: &forms.CreateItemRequest{
				Item:     item,
				Location: loc,
			},
		})
	}

	_, err = r.formsSvc.Forms.BatchUpdate(form.FormId, &forms.BatchUpdateFormRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return service.GoogleFormRes{}, fmt.Errorf("gform: batch update: %w", err)
	}

	return service.GoogleFormRes{FormLink: form.ResponderUri, FormID: form.FormId}, nil
}
