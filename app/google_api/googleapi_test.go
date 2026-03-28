package googleapi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// mockServiceAccount mirrors the fields google.CredentialsFromJSONWithType expects.
type mockServiceAccount struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// generateValidServiceAccountJSON creates a well-formed service-account JSON
// with a freshly generated RSA key so no real credentials are needed.
func generateValidServiceAccountJSON(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	sa := mockServiceAccount{
		Type:                    "service_account",
		ProjectID:               "test-project",
		PrivateKeyID:            "test-key-id",
		PrivateKey:              string(pemKey),
		ClientEmail:             "test@test-project.iam.gserviceaccount.com",
		ClientID:                "123456789",
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/test",
	}

	data, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("failed to marshal mock service account: %v", err)
	}

	return data
}

// chdirTemp changes the working directory to a fresh temp dir for the duration
// of the test and restores it via t.Cleanup. It returns the temp dir path.
func chdirTemp(t *testing.T) string {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	return tmpDir
}

// resetCreds saves the current package-level creds and restores it after the test.
func resetCreds(t *testing.T) {
	t.Helper()
	orig := creds
	t.Cleanup(func() { creds = orig })
	creds = nil
}

// TestInit_FileNotFound verifies that Init returns a non-nil error and leaves
// creds as nil when service-account.json is absent.
func TestInit_FileNotFound(t *testing.T) {
	resetCreds(t)
	chdirTemp(t) // temp dir has no service-account.json

	err := Init(context.Background())

	if err == nil {
		t.Error("expected a non-nil error when service-account.json is absent, got nil")
	}
	if creds != nil {
		t.Errorf("expected creds to be nil when service-account.json is absent, got %v", creds)
	}
}

// TestInit_InvalidJSON verifies that Init returns a non-nil error and leaves
// creds as nil when the JSON content is malformed.
func TestInit_InvalidJSON(t *testing.T) {
	resetCreds(t)
	tmpDir := chdirTemp(t)

	if writeErr := os.WriteFile(filepath.Join(tmpDir, "service-account.json"), []byte("{invalid json{{"), 0600); writeErr != nil {
		t.Fatalf("failed to write invalid JSON file: %v", writeErr)
	}

	err := Init(context.Background())

	if err == nil {
		t.Error("expected a non-nil error for malformed JSON, got nil")
	}
	if creds != nil {
		t.Errorf("expected creds to be nil with invalid JSON, got %v", creds)
	}
}

// TestInit_EmptyJSON verifies that Init returns a non-nil error and does not
// panic when the service-account.json file is empty.
func TestInit_EmptyJSON(t *testing.T) {
	resetCreds(t)
	tmpDir := chdirTemp(t)

	if writeErr := os.WriteFile(filepath.Join(tmpDir, "service-account.json"), []byte{}, 0600); writeErr != nil {
		t.Fatalf("failed to write empty file: %v", writeErr)
	}

	err := Init(context.Background())

	if err == nil {
		t.Error("expected a non-nil error for an empty JSON file, got nil")
	}
	if creds != nil {
		t.Errorf("expected creds to be nil for empty JSON, got %v", creds)
	}
}

// TestInit_ValidServiceAccount verifies that Init returns nil error and sets
// creds when given a well-formed service-account.json.
func TestInit_ValidServiceAccount(t *testing.T) {
	resetCreds(t)
	tmpDir := chdirTemp(t)

	saJSON := generateValidServiceAccountJSON(t)
	if writeErr := os.WriteFile(filepath.Join(tmpDir, "service-account.json"), saJSON, 0600); writeErr != nil {
		t.Fatalf("failed to write service-account.json: %v", writeErr)
	}

	err := Init(context.Background())

	if err != nil {
		t.Errorf("expected nil error for a valid service account, got: %v", err)
	}
	if creds == nil {
		t.Error("expected creds to be non-nil after successful Init, got nil")
	}
}

// TestInit_CredentialsProjectID verifies that creds contains the project ID
// from the service account JSON after a successful Init.
func TestInit_CredentialsProjectID(t *testing.T) {
	resetCreds(t)
	tmpDir := chdirTemp(t)

	saJSON := generateValidServiceAccountJSON(t)
	if writeErr := os.WriteFile(filepath.Join(tmpDir, "service-account.json"), saJSON, 0600); writeErr != nil {
		t.Fatalf("failed to write service-account.json: %v", writeErr)
	}

	err := Init(context.Background())

	if err != nil {
		t.Fatalf("expected nil error for a valid service account, got: %v", err)
	}
	if creds == nil {
		t.Fatal("expected creds to be non-nil after successful Init, got nil")
	}

	const wantProjectID = "test-project"
	if creds.ProjectID != wantProjectID {
		t.Errorf("expected ProjectID %q, got %q", wantProjectID, creds.ProjectID)
	}
}

// TestInit_CredentialsJSONPreserved verifies that the raw JSON bytes are
// preserved in creds after a successful Init.
func TestInit_CredentialsJSONPreserved(t *testing.T) {
	resetCreds(t)
	tmpDir := chdirTemp(t)

	saJSON := generateValidServiceAccountJSON(t)
	if writeErr := os.WriteFile(filepath.Join(tmpDir, "service-account.json"), saJSON, 0600); writeErr != nil {
		t.Fatalf("failed to write service-account.json: %v", writeErr)
	}

	err := Init(context.Background())

	if err != nil {
		t.Fatalf("expected nil error for a valid service account, got: %v", err)
	}
	if creds == nil {
		t.Fatal("expected creds to be non-nil after successful Init, got nil")
	}
	if len(creds.JSON) == 0 {
		t.Error("expected creds.JSON to be non-empty, but it was empty")
	}
}
