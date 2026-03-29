package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/forms/v1"

	googleapi "github.com/arya-bhanu/go-doc-generator/app/google_api"
)

// driveFileID is the Google Drive file we want to verify access to.
const driveFileID = "1DfaDlEM21aGggs5EnmzwLAuNpFNBswnW"

// AppServices mirrors the service registry assembled in cmd/main.go.
type AppServices struct {
	SupabaseDBService *pgxpool.Pool
	GdriveService     *drive.Service
	GFormService      *forms.Service
}

// projectRoot returns the absolute path of the repository root by walking three
// directories up from this source file:
//
//	app/core/documents/service_test.go  →  ../../..  →  <repo root>
func projectRoot(t *testing.T) string {
	t.Helper()
	_, srcFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed: cannot determine source file path")
	}
	// srcFile == .../app/core/documents/service_test.go
	return filepath.Join(filepath.Dir(srcFile), "..", "..", "..")
}

// TestGDriveConnection_AccessFile is a real integration test that verifies the
// application can:
//
//  1. Load credentials from service-account.json.
//  2. Initialise both the Drive and Forms Google API clients.
//  3. Assemble an AppServices struct (identical pattern to cmd/main.go).
//  4. Reach the Drive API and fetch metadata for the known file ID.
func TestGDriveConnection_AccessFile(t *testing.T) {
	ctx := context.Background()

	// ── 1. Switch cwd to project root so googleapi.Init finds service-account.json ──
	root := projectRoot(t)
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	// ── 2. Initialise Google credentials ─────────────────────────────────────
	if err := googleapi.Init(ctx); err != nil {
		t.Fatalf("googleapi.Init: %v", err)
	}

	gdriveService, err := googleapi.InitGDrive(ctx)
	if err != nil {
		t.Fatalf("googleapi.InitGDrive: %v", err)
	}

	gformService, err := googleapi.InitGForm(ctx)
	if err != nil {
		t.Fatalf("googleapi.InitGForm: %v", err)
	}

	// ── 3. Build AppServices – same pattern as cmd/main.go ───────────────────
	appServices := AppServices{
		SupabaseDBService: nil, // DB not required for Drive connectivity check
		GdriveService:     gdriveService,
		GFormService:      gformService,
	}

	// ── 4. Ping Drive API: fetch metadata for the target file ────────────────
	file, err := appServices.GdriveService.Files.Get(driveFileID).
		Fields("id", "name", "mimeType", "size").
		Do()
	if err != nil {
		t.Fatalf("Drive.Files.Get(%q) failed – check service-account permissions: %v", driveFileID, err)
	}

	t.Logf("Google Drive API reachable")
	t.Logf("  id       = %s", file.Id)
	t.Logf("  name     = %s", file.Name)
	t.Logf("  mimeType = %s", file.MimeType)
	t.Logf("  size     = %d bytes", file.Size)

	if file.Id != driveFileID {
		t.Errorf("file ID mismatch: want %q, got %q", driveFileID, file.Id)
	}
}
