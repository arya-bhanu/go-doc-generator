package repository

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/api/forms/v1"
	"google.golang.org/api/option"
)

// roundTripFunc is an adapter that turns a plain function into an http.RoundTripper.
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// jsonResponse builds an *http.Response that carries a JSON body.
func jsonResponse(statusCode int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(string(b))),
		Header:     make(http.Header),
	}
}

// errorResponse returns a Google-style error JSON response.
func errorResponse(statusCode int, message string) *http.Response {
	body := map[string]any{
		"error": map[string]any{
			"code":    statusCode,
			"message": message,
			"status":  "PERMISSION_DENIED",
		},
	}
	return jsonResponse(statusCode, body)
}

// newFormsService creates a *forms.Service backed by the given RoundTripper so
// no real network calls are made during tests.
func newFormsService(t *testing.T, rt http.RoundTripper) *forms.Service {
	t.Helper()
	svc, err := forms.NewService(
		context.Background(),
		option.WithHTTPClient(&http.Client{Transport: rt}),
	)
	if err != nil {
		t.Fatalf("failed to create forms service: %v", err)
	}
	return svc
}

// ─── Create (no items) ────────────────────────────────────────────────────────

// TestCreateForm_NoItems_Success verifies that CreateForm returns the
// ResponderUri when the API call succeeds and no items are provided.
func TestCreateForm_NoItems_Success(t *testing.T) {
	const wantURI = "https://docs.google.com/forms/d/test-form-id/viewform"

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Only the Create endpoint should be called.
		if !strings.Contains(req.URL.Path, "/forms") {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		return jsonResponse(200, &forms.Form{
			FormId:       "test-form-id",
			ResponderUri: wantURI,
			Info:         &forms.Info{Title: "Test Form"},
		}), nil
	})

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	got, err := repo.CreateForm(context.Background(), "Test Form", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FormLink != wantURI {
		t.Errorf("want FormLink %q, got %q", wantURI, got.FormLink)
	}
	if got.FormID != "test-form-id" {
		t.Errorf("want FormID %q, got %q", "test-form-id", got.FormID)
	}
}

// TestCreateForm_WithItems_Success verifies that CreateForm calls BatchUpdate
// when items are provided and still returns the original ResponderUri.
func TestCreateForm_WithItems_Success(t *testing.T) {
	const wantURI = "https://docs.google.com/forms/d/test-form-id/viewform"

	callCount := 0
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch {
		case req.Method == http.MethodPost && !strings.Contains(req.URL.Path, "batchUpdate"):
			// First call: Forms.Create
			return jsonResponse(200, &forms.Form{
				FormId:       "test-form-id",
				ResponderUri: wantURI,
				Info:         &forms.Info{Title: "Test Form"},
			}), nil

		case strings.Contains(req.URL.Path, "batchUpdate"):
			// Second call: Forms.BatchUpdate
			return jsonResponse(200, &forms.BatchUpdateFormResponse{}), nil

		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return errorResponse(404, "not found"), nil
		}
	})

	items := []*forms.Item{
		{Title: "Question 1", ItemId: "item-1"},
	}

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	got, err := repo.CreateForm(context.Background(), "Test Form", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FormLink != wantURI {
		t.Errorf("want FormLink %q, got %q", wantURI, got.FormLink)
	}
	if got.FormID != "test-form-id" {
		t.Errorf("want FormID %q, got %q", "test-form-id", got.FormID)
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (Create + BatchUpdate), got %d", callCount)
	}
}

// ─── Permission / auth errors ─────────────────────────────────────────────────

// TestCreateForm_PermissionDenied verifies that a 403 from the Google API is
// wrapped and returned as a non-nil error.  This is the key scenario the task
// description asks about – "apakah benar-benar dapat permission dari Google".
func TestCreateForm_PermissionDenied(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return errorResponse(403, "Request had insufficient authentication scopes."), nil
	})

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	_, err := repo.CreateForm(context.Background(), "Test Form", nil)
	if err == nil {
		t.Fatal("expected an error for 403 permission-denied, got nil")
	}
	if !strings.Contains(err.Error(), "gform: create form") {
		t.Errorf("expected error to be wrapped with 'gform: create form', got: %v", err)
	}
}

// TestCreateForm_Unauthenticated verifies that a 401 from the Google API is
// wrapped and returned as a non-nil error.
func TestCreateForm_Unauthenticated(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return errorResponse(401, "Request is missing required authentication credential."), nil
	})

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	_, err := repo.CreateForm(context.Background(), "Test Form", nil)
	if err == nil {
		t.Fatal("expected an error for 401 unauthenticated, got nil")
	}
	if !strings.Contains(err.Error(), "gform: create form") {
		t.Errorf("expected error to be wrapped with 'gform: create form', got: %v", err)
	}
}

// ─── Network-level errors ─────────────────────────────────────────────────────

// TestCreateForm_NetworkError verifies that a transport-level error is wrapped
// and returned.
func TestCreateForm_NetworkError(t *testing.T) {
	rt := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	_, err := repo.CreateForm(context.Background(), "Test Form", nil)
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
	if !strings.Contains(err.Error(), "gform: create form") {
		t.Errorf("expected error to be wrapped with 'gform: create form', got: %v", err)
	}
}

// TestCreateForm_BatchUpdateError verifies that a failure in BatchUpdate is
// wrapped with the expected prefix.
func TestCreateForm_BatchUpdateError(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "batchUpdate") {
			return errorResponse(403, "Insufficient permission to edit this form."), nil
		}
		return jsonResponse(200, &forms.Form{
			FormId:       "test-form-id",
			ResponderUri: "https://docs.google.com/forms/d/test-form-id/viewform",
			Info:         &forms.Info{Title: "Test Form"},
		}), nil
	})

	items := []*forms.Item{{Title: "Q1", ItemId: "item-1"}}
	repo := NewGFormRepo(newFormsService(t, rt), nil)
	_, err := repo.CreateForm(context.Background(), "Test Form", items)
	if err == nil {
		t.Fatal("expected error for BatchUpdate failure, got nil")
	}
	if !strings.Contains(err.Error(), "gform: batch update") {
		t.Errorf("expected error to be wrapped with 'gform: batch update', got: %v", err)
	}
}

// ─── Context cancellation ─────────────────────────────────────────────────────

// TestCreateForm_CancelledContext verifies that a cancelled context causes
// CreateForm to return an error.
func TestCreateForm_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Honour the already-cancelled context.
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		return jsonResponse(200, &forms.Form{
			FormId:       "test-form-id",
			ResponderUri: "https://docs.google.com/forms/d/test-form-id/viewform",
		}), nil
	})

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	_, err := repo.CreateForm(ctx, "Test Form", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// ─── Multiple items ───────────────────────────────────────────────────────────

// TestCreateForm_MultipleItems_BatchUpdatePayload verifies that all items are
// forwarded to the BatchUpdate request body.
func TestCreateForm_MultipleItems_BatchUpdatePayload(t *testing.T) {
	items := []*forms.Item{
		{Title: "Q1", ItemId: "item-1"},
		{Title: "Q2", ItemId: "item-2"},
		{Title: "Q3", ItemId: "item-3"},
	}

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "batchUpdate") {
			// Decode the payload and verify the number of requests.
			var payload forms.BatchUpdateFormRequest
			body, _ := io.ReadAll(req.Body)
			if jsonErr := json.Unmarshal(body, &payload); jsonErr != nil {
				t.Errorf("failed to decode BatchUpdate body: %v", jsonErr)
			}
			if len(payload.Requests) != len(items) {
				t.Errorf("expected %d requests in BatchUpdate, got %d", len(items), len(payload.Requests))
			}
			return jsonResponse(200, &forms.BatchUpdateFormResponse{}), nil
		}
		return jsonResponse(200, &forms.Form{
			FormId:       "multi-item-form",
			ResponderUri: "https://docs.google.com/forms/d/multi-item-form/viewform",
			Info:         &forms.Info{Title: "Multi Item Form"},
		}), nil
	})

	repo := NewGFormRepo(newFormsService(t, rt), nil)
	_, err := repo.CreateForm(context.Background(), "Multi Item Form", items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
