package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/arya-bhanu/go-doc-generator/app/conpool"
	"github.com/arya-bhanu/go-doc-generator/app/core/documents"
	"github.com/arya-bhanu/go-doc-generator/app/database"
)

// CreateFormSessions inserts a new row into the form_sessions table.
// doc_details and form_scaffold_cust are stored as JSON.
// form_link is left empty on initial creation.
func CreateFormSessions(payload documents.FormSessions) error {
	docDetailsJSON, err := json.Marshal(payload.DocDetails)
	if err != nil {
		return fmt.Errorf("supabase: marshal doc_details: %w", err)
	}

	formScaffoldCustJSON, err := json.Marshal(payload.FormScaffoldCust)
	fmt.Printf("payload.FormScaffoldCust %+v\n", payload.FormScaffoldCust)
	if err != nil {
		return fmt.Errorf("supabase: marshal form_scaffold_cust: %w", err)
	}

	_, err = database.DB.Exec(
		context.Background(),
		`INSERT INTO form_sessions
			(doc_details, form_link, form_scaffold_cust, user_id, form_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		docDetailsJSON,
		payload.FormLink,
		formScaffoldCustJSON,
		payload.UserID,
		payload.FormID,
	)
	if err != nil {
		return fmt.Errorf("supabase: insert form_sessions: %w", err)
	}

	return nil
}

// FetchFormSession retrieves the form_sessions row whose form_id matches the
// given value. The JSONB columns (doc_details, form_scaffold_cust) are decoded
// into their respective Go types.
func FetchFormSession(formID string) (*documents.FormSessions, error) {
	var (
		session              documents.FormSessions
		docDetailsJSON       []byte
		formScaffoldCustJSON []byte
	)

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT doc_details, form_link, form_scaffold_cust, user_id, form_id
		 FROM form_sessions
		 WHERE form_id = $1
		 LIMIT 1`,
		formID,
	).Scan(
		&docDetailsJSON,
		&session.FormLink,
		&formScaffoldCustJSON,
		&session.UserID,
		&session.FormID,
	)
	if err != nil {
		return nil, fmt.Errorf("supabase: fetch form session %q: %w", formID, err)
	}

	if err := json.Unmarshal(docDetailsJSON, &session.DocDetails); err != nil {
		return nil, fmt.Errorf("supabase: unmarshal doc_details: %w", err)
	}

	if len(formScaffoldCustJSON) > 0 {
		if err := json.Unmarshal(formScaffoldCustJSON, &session.FormScaffoldCust); err != nil {
			return nil, fmt.Errorf("supabase: unmarshal form_scaffold_cust: %w", err)
		}
	}

	return &session, nil
}

// FetchFormSessionByUserID checks whether a form_sessions row already exists
// for the given userID. Returns (session, nil) when found, (nil, nil) when no
// row exists, and (nil, err) on a query/decode error.
func FetchFormSessionByUserID(userID int) (*documents.FormSessions, error) {
	var (
		session              documents.FormSessions
		docDetailsJSON       []byte
		formScaffoldCustJSON []byte
	)

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT doc_details, form_link, form_scaffold_cust, user_id, form_id
		 FROM form_sessions
		 WHERE user_id = $1
		 LIMIT 1`,
		userID,
	).Scan(
		&docDetailsJSON,
		&session.FormLink,
		&formScaffoldCustJSON,
		&session.UserID,
		&session.FormID,
	)
	if err != nil {
		// pgx returns pgx.ErrNoRows when nothing is found – treat it as "not found"
		// rather than a hard error so the caller can decide to create instead.
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("supabase: fetch form session by user_id %d: %w", userID, err)
	}

	if err := json.Unmarshal(docDetailsJSON, &session.DocDetails); err != nil {
		return nil, fmt.Errorf("supabase: unmarshal doc_details: %w", err)
	}
	// form_scaffold_cust is nullable — skip unmarshal when the column is NULL.
	if len(formScaffoldCustJSON) > 0 {
		if err := json.Unmarshal(formScaffoldCustJSON, &session.FormScaffoldCust); err != nil {
			return nil, fmt.Errorf("supabase: unmarshal form_scaffold_cust: %w", err)
		}
	}

	return &session, nil
}

// UpdateFormSession updates the form_link, form_scaffold_cust, doc_details, and
// form_id columns of the form_sessions row that belongs to userID.
func UpdateFormSession(userID int, payload documents.FormSessions) error {
	docDetailsJSON, err := json.Marshal(payload.DocDetails)
	if err != nil {
		return fmt.Errorf("supabase: marshal doc_details: %w", err)
	}

	formScaffoldCustJSON, err := json.Marshal(payload.FormScaffoldCust)
	if err != nil {
		return fmt.Errorf("supabase: marshal form_scaffold_cust: %w", err)
	}

	slog.Info("[UpdateFormSession] docDetailsJSON: ", "docDetailsJSON", docDetailsJSON)

	_, err = database.DB.Exec(
		context.Background(),
		`UPDATE form_sessions
		 SET form_link         = $1,
		     form_scaffold_cust = $2,
		     doc_details        = $3,
		     form_id            = $4
		 WHERE user_id = $5`,
		payload.FormLink,
		formScaffoldCustJSON,
		docDetailsJSON,
		payload.FormID,
		userID,
	)
	if err != nil {
		return fmt.Errorf("supabase: update form_sessions for user_id %d: %w", userID, err)
	}

	return nil
}

// FetchFormFilledCustomer retrieves the current form_filled_customer JSON column
// for the form_sessions row identified by formID and decodes it into a
// map[string]conpool.FormAnswer. Returns an empty (non-nil) map when the column
// is NULL or the row has no data yet.
func FetchFormFilledCustomer(formID string) (map[string]conpool.FormAnswer, error) {
	var raw []byte

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT form_filled_customer
		 FROM form_sessions
		 WHERE form_id = $1
		 LIMIT 1`,
		formID,
	).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("supabase: fetch form_filled_customer %q: %w", formID, err)
	}

	result := make(map[string]conpool.FormAnswer)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, fmt.Errorf("supabase: unmarshal form_filled_customer: %w", err)
		}
	}

	return result, nil
}

// StoreFormFilledCustomer marshals qAndA to JSON and writes it into the
// form_filled_customer column of the form_sessions row identified by formID.
func StoreFormFilledCustomer(formID string, qAndA map[string]conpool.FormAnswer) error {
	data, err := json.Marshal(qAndA)
	if err != nil {
		return fmt.Errorf("supabase: marshal form_filled_customer: %w", err)
	}

	_, err = database.DB.Exec(
		context.Background(),
		`UPDATE form_sessions
		 SET form_filled_customer = $1
		 WHERE form_id = $2`,
		data,
		formID,
	)
	if err != nil {
		return fmt.Errorf("supabase: update form_filled_customer: %w", err)
	}

	return nil
}

// DeleteFormIDSession sets form_id to NULL on the form_sessions row identified
// by formID, effectively unlinking the Google Form from the session.
func DeleteFormIDSession(formID string) error {
	_, err := database.DB.Exec(
		context.Background(),
		`UPDATE form_sessions SET form_id = NULL WHERE form_id = $1`,
		formID,
	)
	if err != nil {
		return fmt.Errorf("supabase: clear form_id for session %q: %w", formID, err)
	}
	return nil
}

// DeleteFormSessionByUserID removes the form_sessions row for the given userID.
func DeleteFormSessionByUserID(userID int) error {
	_, err := database.DB.Exec(
		context.Background(),
		`DELETE FROM form_sessions WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("supabase: delete form_sessions for user_id %d: %w", userID, err)
	}
	return nil
}

// ReplaceDocumentTemplates replaces the entire stored_document_templates table
// with the provided slice inside a single transaction. It first deletes all
// existing rows, then inserts the new ones. If any insert fails the delete is
// rolled back, leaving the table unchanged.
func ReplaceDocumentTemplates(templates []documents.StoredDocumentTemplate) error {
	ctx := context.Background()

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("supabase: begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback on any early return

	_, err = tx.Exec(ctx, `DELETE FROM stored_document_templates`)
	if err != nil {
		return fmt.Errorf("supabase: delete stored_document_templates: %w", err)
	}

	for _, t := range templates {
		_, err = tx.Exec(
			ctx,
			`INSERT INTO stored_document_templates (google_file_id, link, title)
			 VALUES ($1, $2, $3)`,
			t.GoogleFileID,
			t.Link,
			t.Title,
		)
		if err != nil {
			return fmt.Errorf("supabase: insert stored_document_templates %q: %w", t.GoogleFileID, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("supabase: commit transaction: %w", err)
	}

	return nil
}

// FetchDocVariable looks up a single row in document_variables whose variable
// column matches the given value. It returns nil when the row is not found or
// a database error occurs, which marshals as JSON null.
func FetchDocVariable(variable string) *documents.DocumentVariable {
	var dv documents.DocumentVariable

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT id, variable, label, created_at, description, "type", is_fill_customers
		 FROM document_variables
		 WHERE variable = $1
		 LIMIT 1`,
		variable,
	).Scan(
		&dv.ID,
		&dv.Variable,
		&dv.Label,
		&dv.CreatedAt,
		&dv.Description,
		&dv.Type,
		&dv.IsFillCustomers,
	)
	if err != nil {
		// Return nil on not-found or any other error — marshals as JSON null.
		return nil
	}

	return &dv
}

// FetchAnswerdCustomerForm queries the form_sessions row for the given userID
// and returns the form_filled_customer column decoded as map[string]conpool.FormAnswer.
// Returns nil if no matching row is found or an error occurs.
func FetchAnswerdCustomerForm(userID int) map[string]conpool.FormAnswer {
	var formFilledCustomerJSON []byte

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT form_filled_customer
		 FROM form_sessions
		 WHERE user_id = $1
		 LIMIT 1`,
		userID,
	).Scan(&formFilledCustomerJSON)
	if err != nil {
		return nil
	}

	var result map[string]conpool.FormAnswer
	if err := json.Unmarshal(formFilledCustomerJSON, &result); err != nil {
		return nil
	}

	return result
}
