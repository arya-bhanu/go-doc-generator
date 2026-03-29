package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

	slog.Info("formScaffoldCustJSON", "val", string(formScaffoldCustJSON))

	_, err = database.DB.Exec(
		context.Background(),
		`INSERT INTO form_sessions
			(doc_details, form_link, form_scaffold_cust, form_scaffold_ops, user_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		docDetailsJSON,
		payload.FormLink,
		formScaffoldCustJSON,
		formScaffoldOpsJSON,
		payload.UserID,
	)
	if err != nil {
		return fmt.Errorf("supabase: insert form_sessions: %w", err)
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
