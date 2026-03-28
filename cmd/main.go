package main

import (
	"log"

	"github.com/joho/godotenv"

	"github.com/arya-bhanu/go-doc-generator/app/database"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	database.Connect()
	defer database.Close()
}
