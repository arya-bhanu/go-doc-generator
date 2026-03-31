package service

import (
	"context"

	"google.golang.org/api/forms/v1"

	"github.com/arya-bhanu/go-doc-generator/app/core/documents"
	formconst "github.com/arya-bhanu/go-doc-generator/constants/form_const"
)

type FormRepository interface {
	CreateForm(ctx context.Context, title string, items []*forms.Item) (GoogleFormRes, error)
}

type FormService struct {
	repo FormRepository
}

type GoogleFormRes struct {
	FormLink string
	FormID   string
}

func NewFormService(repo FormRepository) *FormService {
	return &FormService{repo: repo}
}

func (s *FormService) GenerateGoogleForm(ctx context.Context, formTitle string, vars map[string]*documents.DocumentVariable) (GoogleFormRes, error) {
	items := make([]*forms.Item, 0, len(vars))

	for key := range vars {
		docVar := vars[key]
		if docVar == nil {
			continue
		}

		fieldLabel := docVar.Label

		var docVarType string
		if docVar.Type != nil {
			docVarType = *docVar.Type
		}

		var item *forms.Item

		switch docVarType {
		case "", formconst.ChoiceQuestionShort:
			item = generateTextShortQuestion(fieldLabel)
		case formconst.ChoiceQuestionCheckbox:
			item = generateCheckboxQuestion(fieldLabel)
		case formconst.ChoiceQuestionRadio:
			item = generateRadioQuestion(fieldLabel)
		case formconst.ChoiceQuestionDropdown:
			item = generateDropdownQuestion(fieldLabel)
		case formconst.ChoiceQuestionLong:
			item = generateTextLongQuestion(fieldLabel)
		case formconst.ScaleQuestion:
			item = generateScaleQuestion(fieldLabel)
		case formconst.DateQuestion:
			item = generateDateQuestion(fieldLabel)
		case formconst.TimeQuestion:
			item = generateTimeQuestion(fieldLabel)
		}

		if item != nil {
			items = append(items, item)
		}
	}

	return s.repo.CreateForm(ctx, formTitle, items)
}

func generateRadioQuestion(label string) *forms.Item {
	return nil // TODO: implement
}

func generateCheckboxQuestion(label string) *forms.Item {
	return nil // TODO: implement
}

func generateDropdownQuestion(label string) *forms.Item {
	return nil // TODO: implement
}

func generateTextShortQuestion(label string) *forms.Item {
	return &forms.Item{
		Title: label,
		QuestionItem: &forms.QuestionItem{
			Question: &forms.Question{
				TextQuestion: &forms.TextQuestion{
					Paragraph: false,
				},
			},
		},
	}
}

func generateTextLongQuestion(label string) *forms.Item {
	return &forms.Item{
		Title: label,
		QuestionItem: &forms.QuestionItem{
			Question: &forms.Question{
				TextQuestion: &forms.TextQuestion{
					Paragraph: true,
				},
			},
		},
	}
}

func generateScaleQuestion(label string) *forms.Item {
	return nil // TODO: implement
}

func generateDateQuestion(label string) *forms.Item {
	return nil // TODO: implement
}

func generateTimeQuestion(label string) *forms.Item {
	return nil // TODO: implement
}
