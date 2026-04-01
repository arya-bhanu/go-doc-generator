package service

import (
	"github.com/arya-bhanu/go-doc-generator/app/conpool"
	documents "github.com/arya-bhanu/go-doc-generator/app/core/documents"
	docrepo "github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
)

type UserOpsField struct {
	Question string   `json:"question"`
	Answers  []string `json:"answers"`
	Variable string   `json:"variable"`
	Type     *string  `json:"type"`
}

func fetchAnsweredFilledOps(userID int) map[string]conpool.FormAnswer {
	return docrepo.FetchFormFilledOps(userID)
}

func fetchOldFormScaffoldOps(userID int) *map[string]*documents.DocumentVariable {
	return docrepo.FetchFormScaffoldOps(userID)
}

func GenerateUserOpsField(userID int, userOpsVariable map[string]*documents.DocumentVariable) map[string]UserOpsField {
	prefilled := make(map[string]UserOpsField)
	answeredOpsQuestions := fetchAnsweredFilledOps(userID)
	oldOpsFormScaffold := fetchOldFormScaffoldOps(userID)

	for key, val := range answeredOpsQuestions {
		var typeForm *string

		if oldOpsFormScaffold != nil {
			val, ok := (*oldOpsFormScaffold)[key]
			if ok && val != nil {
				typeForm = (*val).Type
			}
		}

		prefilled[key] = UserOpsField{
			Variable: key,
			Question: val.Question,
			Answers:  val.Answers,
			Type:     typeForm,
		}

		delete(userOpsVariable, key)
	}

	for key, val := range userOpsVariable {
		prefilled[key] = UserOpsField{
			Question: val.Label,
			Answers:  []string{""},
			Variable: val.Variable,
			Type:     val.Type,
		}
	}

	return prefilled
}
