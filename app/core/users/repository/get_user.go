package repository

import (
	"context"
	"encoding/json"

	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	"github.com/arya-bhanu/go-doc-generator/app/database"
)

// GetUserWithEmail looks up a user in the ops_user table by their email address.
// It returns the matching UserOps and a nil error on success.
// If no row is found it returns pgx.ErrNoRows so callers can distinguish
// "not found" from other database errors.
func GetUserWithEmail(email string) (users.UserOps, error) {
	var user users.UserOps
	err := database.DB.QueryRow(
		context.Background(),
		"SELECT id, email FROM ops_user WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Email)
	if err != nil {
		return users.UserOps{}, err
	}
	return user, nil
}

// GetUserByID looks up a user in the ops_user table by their numeric ID.
// It returns the matching UserOps and a nil error on success.
// If no row is found it returns pgx.ErrNoRows.
func GetUserByID(id int) (users.UserOps, error) {
	var user users.UserOps
	err := database.DB.QueryRow(
		context.Background(),
		"SELECT id, email FROM ops_user WHERE id = $1",
		id,
	).Scan(&user.ID, &user.Email)
	if err != nil {
		return users.UserOps{}, err
	}
	return user, nil
}

// FetchOpsUserFormFilled fetches the form_filled JSONB column from the ops_user
// row whose id matches userID and decodes it into a map[string]users.OpsUserDataField.
// Returns nil when no row is found or the column is NULL/empty.
func FetchOpsUserFormFilled(userID int) map[string]users.OpsUserDataField {
	var raw []byte

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT form_filled FROM ops_user WHERE id = $1 LIMIT 1`,
		userID,
	).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return nil
	}

	var result map[string]users.OpsUserDataField
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}

	return result
}

// UpdateOpsUserFormFilled serialises formFilled as JSONB and writes it into the
// form_filled column of the ops_user row identified by userID.
func UpdateOpsUserFormFilled(userID int, formFilled map[string]users.OpsUserDataField) error {
	raw, err := json.Marshal(formFilled)
	if err != nil {
		return err
	}

	_, err = database.DB.Exec(
		context.Background(),
		`UPDATE ops_user SET form_filled = $1 WHERE id = $2`,
		raw,
		userID,
	)
	return err
}

// FetchDocumentVariablesByOps retrieves all rows from document_variables where
// is_fill_customers is FALSE and returns them as a slice of DocumentVariablesTable.
// prefilled_value defaults to an empty string when the column is NULL.
func FetchDocumentVariablesByOps() ([]users.DocumentVariablesTable, error) {
	rows, err := database.DB.Query(
		context.Background(),
		`SELECT id, variable, label,
		        COALESCE(description, ''),
		        COALESCE("type", ''),
		        is_fill_customers,
		        COALESCE(prefilled_value, '')
		 FROM document_variables
		 WHERE is_fill_customers = FALSE`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []users.DocumentVariablesTable
	for rows.Next() {
		var dv users.DocumentVariablesTable
		if err := rows.Scan(
			&dv.ID,
			&dv.Variable,
			&dv.Label,
			&dv.Description,
			&dv.Type,
			&dv.IsFillCustomers,
			&dv.PrefilledValue,
		); err != nil {
			return nil, err
		}
		result = append(result, dv)
	}

	return result, nil
}
