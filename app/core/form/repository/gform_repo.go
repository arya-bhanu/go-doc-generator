package repository

import (
	"context"
	"fmt"

	"google.golang.org/api/forms/v1"
)

type GFormRepository interface {
	CreateForm(ctx context.Context, title string, items []*forms.Item) (string, error)
}

type GFormRepo struct {
	svc *forms.Service
}

func NewGFormRepo(svc *forms.Service) *GFormRepo {
	return &GFormRepo{svc: svc}
}

func (r *GFormRepo) CreateForm(ctx context.Context, title string, items []*forms.Item) (string, error) {
	form, err := r.svc.Forms.Create(&forms.Form{
		Info: &forms.Info{
			Title: title,
		},
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gform: create form: %w", err)
	}

	if len(items) == 0 {
		return form.ResponderUri, nil
	}

	requests := make([]*forms.Request, 0, len(items))
	for i, item := range items {
		requests = append(requests, &forms.Request{
			CreateItem: &forms.CreateItemRequest{
				Item:     item,
				Location: &forms.Location{Index: int64(i)},
			},
		})
	}

	_, err = r.svc.Forms.BatchUpdate(form.FormId, &forms.BatchUpdateFormRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("gform: batch update: %w", err)
	}

	return form.ResponderUri, nil
}
