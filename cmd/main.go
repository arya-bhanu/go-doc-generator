package main

import (
	"log/slog"

	"github.com/joho/godotenv"

	"github.com/arya-bhanu/go-doc-generator/app/database"
	"github.com/arya-bhanu/go-doc-generator/app/server"
	logwrapper "github.com/arya-bhanu/go-doc-generator/utils/log_wrapper"
)

func main() {
	logwrapper.Init()

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using system environment variables", "error", err)
	}

	database.Connect()
	defer database.Close()

	server.Start()
}
