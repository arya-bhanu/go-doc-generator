package service

import (
	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	usersrepo "github.com/arya-bhanu/go-doc-generator/app/core/users/repository"
)

// FetchOpsField retrieves all document_variables rows where is_fill_customers is FALSE.
func FetchOpsField() []users.DocumentVariablesTable {
	result, err := usersrepo.FetchDocumentVariablesByOps()
	if err != nil {
		return nil
	}
	return result
}

// FetchExistingAnsweredOpsFieldForm fetches the form_filled JSON column from
// the ops_user row identified by userID and returns it as a
// map[string]users.OpsUserDataField. Returns an empty map when no data exists.
func FetchExistingAnsweredOpsFieldForm(userID int) map[string]users.OpsUserDataField {
	result := usersrepo.FetchOpsUserFormFilled(userID)
	if result == nil {
		return make(map[string]users.OpsUserDataField)
	}
	return result
}

// SubmitOpsForm converts the submitted slice of OpsUserDataField into a
// map keyed by Variable and persists it into the ops_user.form_filled column.
func SubmitOpsForm(userID int, fields []users.OpsUserDataField) error {
	formFilled := make(map[string]users.OpsUserDataField, len(fields))
	for _, f := range fields {
		formFilled[f.Variable] = f
	}
	return usersrepo.UpdateOpsUserFormFilled(userID, formFilled)
}

// CleanOpsField converts a slice of DocumentVariablesTable into a
// map[string]string keyed by Variable, using PrefilledValue when available.
func CleanOpsField(opsFields []users.DocumentVariablesTable) map[string]users.OpsUserDataField {
	opsFilledMapping := make(map[string]users.OpsUserDataField)
	for _, val := range opsFields {
		if val.PrefilledValue != "" {
			opsFilledMapping[val.Variable] = users.OpsUserDataField{
				Label:    val.Label,
				Variable: val.Variable,
				Answer:   val.PrefilledValue,
			}
			continue
		}
		opsFilledMapping[val.Variable] = users.OpsUserDataField{
			Label:    val.Label,
			Variable: val.Variable,
			Answer:   "",
		}
	}

	return opsFilledMapping
}
