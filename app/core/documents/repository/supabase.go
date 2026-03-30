package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arya-bhanu/go-doc-generator/app/conpool"
	"github.com/arya-bhanu/go-doc-generator/app/core/documents"
	"github.com/arya-bhanu/go-doc-generator/app/database"
)

// CreateFormSessions inserts a new row into the form_sessions table.
// doc_details, form_scaffold_cust, and form_scaffold_ops are stored as JSON.
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

	formScaffoldOpsJSON, err := json.Marshal(payload.FormScaffoldOps)
	if err != nil {
		return fmt.Errorf("supabase: marshal form_scaffold_ops: %w", err)
	}

	_, err = database.DB.Exec(
		context.Background(),
		`INSERT INTO form_sessions
			(doc_details, form_link, form_scaffold_cust, form_scaffold_ops, user_id, form_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		docDetailsJSON,
		payload.FormLink,
		formScaffoldCustJSON,
		formScaffoldOpsJSON,
		payload.UserID,
		payload.FormID,
	)
	if err != nil {
		return fmt.Errorf("supabase: insert form_sessions: %w", err)
	}

	return nil
}

// FetchFormSession retrieves the form_sessions row whose form_id matches the
// given value.  The JSONB columns (doc_details, form_scaffold_cust,
// form_scaffold_ops) are decoded into their respective Go types.
func FetchFormSession(formID string) (*documents.FormSessions, error) {
	var (
		session              documents.FormSessions
		docDetailsJSON       []byte
		formScaffoldCustJSON []byte
		formScaffoldOpsJSON  []byte
	)

	err := database.DB.QueryRow(
		context.Background(),
		`SELECT doc_details, form_link, form_scaffold_cust, form_scaffold_ops, user_id, form_id
		 FROM form_sessions
		 WHERE form_id = $1
		 LIMIT 1`,
		formID,
	).Scan(
		&docDetailsJSON,
		&session.FormLink,
		&formScaffoldCustJSON,
		&formScaffoldOpsJSON,
		&session.UserID,
		&session.FormID,
	)
	if err != nil {
		return nil, fmt.Errorf("supabase: fetch form session %q: %w", formID, err)
	}

	if err := json.Unmarshal(docDetailsJSON, &session.DocDetails); err != nil {
		return nil, fmt.Errorf("supabase: unmarshal doc_details: %w", err)
	}
	if err := json.Unmarshal(formScaffoldCustJSON, &session.FormScaffoldCust); err != nil {
		return nil, fmt.Errorf("supabase: unmarshal form_scaffold_cust: %w", err)
	}
	if err := json.Unmarshal(formScaffoldOpsJSON, &session.FormScaffoldOps); err != nil {
		return nil, fmt.Errorf("supabase: unmarshal form_scaffold_ops: %w", err)
	}

	return &session, nil
}

// StoreFormFilledCustomer marshals qAndA to JSON and writes it into the
// form_filled_customer column of the form_sessions row identified by formID.
func StoreFormFilledCustomer(formID string, qAndA []conpool.FormAnswer) error {
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
