package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/forms/v1"

	"github.com/arya-bhanu/go-doc-generator/app/conpool"
	ctr "github.com/arya-bhanu/go-doc-generator/app/core/documents/controller"
	docrepo "github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	docsvc "github.com/arya-bhanu/go-doc-generator/app/core/documents/service"
	formrepo "github.com/arya-bhanu/go-doc-generator/app/core/form/repository"
	formsvc "github.com/arya-bhanu/go-doc-generator/app/core/form/service"
	ops_ctr "github.com/arya-bhanu/go-doc-generator/app/core/users/controller"
	"github.com/arya-bhanu/go-doc-generator/app/database"
	googleapi "github.com/arya-bhanu/go-doc-generator/app/google_api"
	"github.com/arya-bhanu/go-doc-generator/app/server"
	logwrapper "github.com/arya-bhanu/go-doc-generator/utils/log_wrapper"
)

// AppServices holds external service clients that require their own
// configuration (credentials, connection pools, etc.).
type AppServices struct {
	SupabaseDBService *pgxpool.Pool
	GdriveService     *drive.Service
	GFormService      *forms.Service
}

var (
	ctx         = context.Background()
	appServices AppServices
	docService  *docsvc.DocumentService
)

func main() {

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using system environment variables", "error", err)
	}

	logwrapper.Init()

	db := database.Connect(ctx)
	defer database.Close()

	// ── Google API Initialisation ─────────────────────────────────────────────
	// Prefer OAuth user credentials (token.json) for Forms & Drive so that:
	//   • forms.Create works (service accounts have 0 Drive quota)
	//   • created forms are owned by the real user and can be moved to a folder
	// Falls back to service-account credentials when token.json is absent.
	var gformService *forms.Service
	var gdriveService *drive.Service

	oauthClient, oauthErr := googleapi.InitOAuthHTTPClient(ctx)
	if oauthErr == nil {
		slog.Info("using OAuth user credentials for Google APIs")
		var err error
		gformService, err = googleapi.InitGFormOAuth(ctx, oauthClient)
		if err != nil {
			slog.Error("failed to init Forms service (OAuth)", "err", err)
			return
		}
		gdriveService, err = googleapi.InitGDriveOAuth(ctx, oauthClient)
		if err != nil {
			slog.Error("failed to init Drive service (OAuth)", "err", err)
			return
		}
	} else {
		slog.Warn("OAuth token not found, falling back to service account", "err", oauthErr)
		if err := googleapi.Init(ctx); err != nil {
			slog.Error("failed to init service account credentials", "err", err)
			return
		}
		var err error
		gdriveService, err = googleapi.InitGDrive(ctx)
		if err != nil {
			slog.Error("failed to init Drive service", "err", err)
			return
		}
		gformService, err = googleapi.InitGForm(ctx)
		if err != nil {
			slog.Error("failed to init Forms service", "err", err)
			return
		}
	}

	appServices = AppServices{
		SupabaseDBService: db,
		GdriveService:     gdriveService,
		GFormService:      gformService,
	}

	gdriveRepo := docrepo.NewGDriveRepo(appServices.GdriveService)
	docService = docsvc.NewDocumentService(gdriveRepo)

	// Pass Drive service + folder ID so CreateForm can move forms automatically
	gFormRepo := formrepo.NewGFormRepo(appServices.GFormService, appServices.GdriveService)
	formService := formsvc.NewFormService(gFormRepo)

	slog.Info("connected to gdriveService")
	slog.Info("connected to gformService")

	// Initialise and start the form-response watcher pooler.
	// conpool.Init must be called after gformService is ready.
	// SetResponseHandler must be registered before StartPooler so that no
	// responses are missed on the very first poll tick.
	conpool.Init(gformService)
	conpool.SetResponseHandler(docService.GenerateDocuments)
	conpool.StartPooler()

	handler := ctr.NewHandler(docService, formService)
	opsHandler := ops_ctr.NewController()
	server.Start(handler, opsHandler)
}
