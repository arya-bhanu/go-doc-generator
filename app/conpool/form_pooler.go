package conpool

import (
	"context"
	"log/slog"
	"time"

	"github.com/arya-bhanu/go-doc-generator/app/database"
)

// FetchFormIDInit is called once at startup.  It queries the form_sessions
// table for every non-empty form_id and loads them into the in-memory map
// so the watcher covers forms that were created before this process started.
func FetchFormIDInit() {
	rows, err := database.DB.Query(
		context.Background(),
		`SELECT form_id FROM form_sessions WHERE form_id IS NOT NULL AND form_id != ''`,
	)
	if err != nil {
		slog.Error("conpool: fetch form IDs init", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var formID string
		if err := rows.Scan(&formID); err != nil {
			slog.Error("conpool: scan form ID", "err", err)
			continue
		}
		// Direct map write – FetchFormIDInit runs before StartPooler (single-threaded).
		storeFormID[formID] = struct{}{}
	}

	slog.Info("conpool: form IDs loaded", "count", len(storeFormID))
}

// DeleteFormID removes a form session from the database AND from the
// in-memory watcher map.
//
//   - If form_session_id == "" → delete every row where form_id matches.
//   - If form_id == ""         → look up the row by id, delete it, and evict
//     the associated form_id from the map.
func DeleteFormID(form_session_id string, form_id string) {
	mu.Lock()
	defer mu.Unlock()

	ctx := context.Background()

	if form_session_id == "" && form_id != "" {
		if _, err := database.DB.Exec(ctx,
			`DELETE FROM form_sessions WHERE form_id = $1`, form_id,
		); err != nil {
			slog.Error("conpool: delete by form_id", "err", err)
			return
		}
		delete(storeFormID, form_id)

	} else if form_id == "" && form_session_id != "" {
		// Look up the form_id first so we can evict it from the map.
		var fid string
		_ = database.DB.QueryRow(ctx,
			`SELECT form_id FROM form_sessions WHERE id = $1`, form_session_id,
		).Scan(&fid)

		if _, err := database.DB.Exec(ctx,
			`DELETE FROM form_sessions WHERE id = $1`, form_session_id,
		); err != nil {
			slog.Error("conpool: delete by session id", "err", err)
			return
		}
		if fid != "" {
			delete(storeFormID, fid)
		}
	}
}

// AddFormID registers a Google Form ID in the in-memory watcher map.
// The map guarantees uniqueness – duplicate IDs are silently ignored.
func AddFormID(formID string) {
	mu.Lock()
	defer mu.Unlock()
	storeFormID[formID] = struct{}{}
}

// watchForms is the background goroutine started by StartPooler.
// On every tick it takes a snapshot of the current form IDs and spawns one
// goroutine per form to poll for new responses.
func watchForms() {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	slog.Info("conpool: form watcher started", "interval", pollInterval)

	for range ticker.C {
		mu.RLock()
		ids := make([]string, 0, len(storeFormID))
		for id := range storeFormID {
			ids = append(ids, id)
		}
		mu.RUnlock()

		for _, id := range ids {
			go pollFormResponses(id)
		}
	}
}

// pollFormResponses fetches the latest responses for a single form from the
// Google Forms API and logs a summary.  Extend / replace the log call to
// store or process responses as needed.
func pollFormResponses(formID string) {
	if formsSvc == nil {
		slog.Error("conpool: forms service not initialised – call conpool.Init first")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := formsSvc.Forms.Responses.List(formID).Context(ctx).Do()
	if err != nil {
		slog.Error("conpool: poll form responses", "formID", formID, "err", err)
		return
	}

	slog.Info("conpool: polled form", "formID", formID, "responses", len(resp.Responses))

	// TODO: process / persist resp.Responses as required.
}
