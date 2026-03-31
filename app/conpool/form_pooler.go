package conpool

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/api/forms/v1"

	"github.com/arya-bhanu/go-doc-generator/app/database"
)

// FormAnswer holds a single question/answer pair from a form response.
type FormAnswer struct {
	Question string   `json:"question"`
	Answers  []string `json:"answers"`
}

// FormResponseResult is the clean, human-readable version of one form submission.
type FormResponseResult struct {
	ResponseID  string       `json:"response_id"`
	SubmittedAt string       `json:"submitted_at"`
	Answers     []FormAnswer `json:"answers"`
}

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

// pollFormResponses fetches the form structure and all current responses for
// a single form, pairs each answer with its question title, and logs the
// clean result.  Replace / extend the log call to persist the data as needed.
func pollFormResponses(formID string) {
	if formsSvc == nil {
		slog.Error("conpool: forms service not initialised – call conpool.Init first")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1. Fetch form structure to get question titles keyed by question ID.
	form, err := formsSvc.Forms.Get(formID).Context(ctx).Do()
	if err != nil {
		slog.Error("conpool: get form structure", "formID", formID, "err", err)
		return
	}
	questionMap := buildQuestionMap(form)

	// 2. Fetch all submitted responses.
	resp, err := formsSvc.Forms.Responses.List(formID).Context(ctx).Do()
	if err != nil {
		slog.Error("conpool: list form responses", "formID", formID, "err", err)
		return
	}

	// 3. Build clean paired result.
	results := parseResponses(questionMap, resp.Responses)

	slog.Info("conpool: polled form", "formID", formID, "responses", len(results))

	// 4. Dispatch every response that hasn't been processed yet.
	//    We hold the write lock only for the map bookkeeping, then release it
	//    before spawning goroutines so the lock duration stays minimal.
	if responseHandler == nil {
		return
	}

	mu.Lock()
	if _, ok := processedResponses[formID]; !ok {
		processedResponses[formID] = make(map[string]struct{})
	}
	var fresh []FormResponseResult
	for _, result := range results {
		if _, seen := processedResponses[formID][result.ResponseID]; seen {
			continue
		}
		processedResponses[formID][result.ResponseID] = struct{}{}
		fresh = append(fresh, result)
	}
	mu.Unlock()

	for _, result := range fresh {
		captured := result // avoid closure capture of loop variable
		go responseHandler(formID, captured.Answers)
	}
}

// buildQuestionMap returns a map of questionID → question title by walking
// the form's item list.  Only items that carry a QuestionItem are included.
func buildQuestionMap(form *forms.Form) map[string]string {
	m := make(map[string]string, len(form.Items))
	for _, item := range form.Items {
		if item.QuestionItem == nil || item.QuestionItem.Question == nil {
			continue
		}
		qid := item.QuestionItem.Question.QuestionId
		if qid != "" {
			m[qid] = item.Title
		}
	}
	return m
}

// parseResponses converts raw Google Forms responses into clean
// []FormResponseResult objects where every answer is paired with its
// question title from questionMap.
func parseResponses(questionMap map[string]string, responses []*forms.FormResponse) []FormResponseResult {
	results := make([]FormResponseResult, 0, len(responses))

	for _, r := range responses {
		result := FormResponseResult{
			ResponseID:  r.ResponseId,
			SubmittedAt: r.LastSubmittedTime,
		}

		for qid, answerObj := range r.Answers {
			title, ok := questionMap[qid]
			if !ok {
				title = qid // fall back to raw ID if title not found
			}

			values := make([]string, 0)
			if answerObj.TextAnswers != nil {
				for _, a := range answerObj.TextAnswers.Answers {
					values = append(values, a.Value)
				}
			}

			result.Answers = append(result.Answers, FormAnswer{
				Question: title,
				Answers:  values,
			})
		}

		results = append(results, result)
	}

	return results
}
