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

	logwrapper.Init()

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using system environment variables", "error", err)
		return
	}

	db := database.Connect(ctx)
	defer database.Close()

	err := googleapi.Init(ctx)
	gdriveService, err := googleapi.InitGDrive(ctx)

	gformService, err := googleapi.InitGForm(ctx)

	appServices = AppServices{
		SupabaseDBService: db,
		GdriveService:     gdriveService,
		GFormService:      gformService,
	}

	gdriveRepo := docrepo.NewGDriveRepo(appServices.GdriveService)
	docService = docsvc.NewDocumentService(gdriveRepo)

	gFormRepo := formrepo.NewGFormRepo(appServices.GFormService)
	formService := formsvc.NewFormService(gFormRepo)

	if err != nil {
		slog.Error("error init google api services", "err", err.Error())
		return
	}

	slog.Info("connected to gdriveService")
	slog.Info("connected to gformService")

	// Initialise and start the form-response watcher pooler.
	// conpool.Init must be called after gformService is ready.
	conpool.Init(gformService)
	conpool.StartPooler()

	handler := ctr.NewHandler(docService, formService)
	server.Start(handler)
}
