// tools/test_gform/main.go
//
// Script ini:
// 1. Load OAuth2 token dari token.json (generated oleh tools/get_token/)
// 2. Copy form template via Google Drive API (files.copy) ke folder target
// 3. Update title & item pertama via Google Forms BatchUpdate
//
// PENTING: Jalankan `go run ./tools/get_token/` terlebih dahulu (sekali saja)
// untuk mendapatkan token.json. Setelah itu script ini bisa dijalankan berulang
// kali TANPA consent screen — refresh_token dipakai secara otomatis.
//
// Usage (jalankan dari ROOT project):
//
//	go run ./tools/test_gform/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/forms/v1"
	"google.golang.org/api/option"
)

const (
	// ID form template (dari URL: /forms/d/<ID>/edit)
	templateFormID = "1qRuiBJwmiQoxGmT6wdQnk3XGSnJIjZlKaxishpa-6ug"

	// ID folder Drive tujuan
	targetFolderID = "1wDPnReVUHSQgA7YS0aWoJgiJniiaIQNT"

	tokenFile = "token.json"
	credsFile = "oauth-client-secret.json"
)

// Scope yang dibutuhkan
var scopes = []string{
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/drive.file",
	"https://www.googleapis.com/auth/forms.body",
	"https://www.googleapis.com/auth/forms.responses.readonly",
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ── 1. Load OAuth2 token ──────────────────────────────────────────────────
	fmt.Println("── Load OAuth2 token ────────────────────────────────────────────")

	credsJSON, err := os.ReadFile(credsFile)
	if err != nil {
		log.Fatalf("❌  Gagal baca %s: %v\n\n"+
			"  Download dari Google Cloud Console:\n"+
			"  APIs & Services → Credentials → OAuth 2.0 Client ID → Desktop App → Download JSON\n"+
			"  Simpan sebagai credentials.json di root project.",
			credsFile, err)
	}

	oauthConfig, err := google.ConfigFromJSON(credsJSON, scopes...)
	if err != nil {
		log.Fatalf("❌  Gagal parse credentials.json: %v", err)
	}
	oauthConfig.RedirectURL = "http://localhost:9999/callback"

	tokenJSON, err := os.ReadFile(tokenFile)
	if err != nil {
		log.Fatalf("❌  Gagal baca %s: %v\n\n"+
			"  Jalankan dulu: go run ./tools/get_token/\n"+
			"  (hanya perlu sekali — consent screen akan muncul sekali saja)",
			tokenFile, err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		log.Fatalf("❌  Gagal parse token.json: %v", err)
	}

	// TokenSource otomatis refresh jika expired — TANPA consent screen
	tokenSource := oauthConfig.TokenSource(ctx, &token)
	httpClient := oauth2.NewClient(ctx, tokenSource)

	fmt.Println("✅  OAuth2 token loaded (refresh otomatis jika expired)")
	fmt.Printf("    Token type : %s\n", token.TokenType)
	fmt.Printf("    Expiry     : %s\n", token.Expiry.Format("2006-01-02 15:04:05"))

	// ── 2. Init Drive & Forms service ─────────────────────────────────────────
	driveSvc, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("❌  Gagal init Drive service: %v", err)
	}

	formsSvc, err := forms.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("❌  Gagal init Forms service: %v", err)
	}

	// Verifikasi quota Drive user (bukan service account — harusnya 15GB+)
	aboutRes, _ := driveSvc.About.Get().Fields("storageQuota, user").Do()
	if aboutRes != nil {
		q := aboutRes.StorageQuota
		fmt.Printf("    Drive user : %s\n", aboutRes.User.EmailAddress)
		fmt.Printf("    Drive quota: limit=%d GB, usage=%d MB\n",
			q.Limit/1024/1024/1024, q.Usage/1024/1024)
	}

	// ── STEP 1: Copy form template ke folder target ───────────────────────────
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Println("  STEP 1 — Copy template form via Drive API")
	fmt.Printf("    Template ID : %s\n", templateFormID)
	fmt.Printf("    Folder ID   : %s\n", targetFolderID)
	fmt.Println("─────────────────────────────────────────────────────────────────")

	newTitle := fmt.Sprintf("[COPY] Form Template — %s", time.Now().Format("2006-01-02 15:04:05"))
	copiedFile, err := driveSvc.Files.Copy(templateFormID, &drive.File{
		Name:    newTitle,
		Parents: []string{targetFolderID},
	}).
		SupportsAllDrives(true).
		Fields("id, name, webViewLink").
		Do()
	if err != nil {
		fmt.Printf("❌  Drive files.copy GAGAL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅  Form berhasil di-copy!\n")
	fmt.Printf("    File ID     : %s\n", copiedFile.Id)
	fmt.Printf("    Name        : %s\n", copiedFile.Name)
	fmt.Printf("    Link        : %s\n", copiedFile.WebViewLink)

	// ── STEP 2: Ambil struktur form hasil copy ────────────────────────────────
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Println("  STEP 2 — Ambil struktur form hasil copy (Forms API)")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	copiedForm, err := formsSvc.Forms.Get(copiedFile.Id).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌  Forms.Get GAGAL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅  Forms.Get berhasil!\n")
	fmt.Printf("    FormId      : %s\n", copiedForm.FormId)
	fmt.Printf("    Title       : %s\n", copiedForm.Info.Title)
	fmt.Printf("    Items       : %d buah\n", len(copiedForm.Items))
	for i, item := range copiedForm.Items {
		fmt.Printf("    [%d] ItemId=%s | Title=%q\n", i, item.ItemId, item.Title)
	}

	// ── STEP 3: Update title & item pertama via BatchUpdate ───────────────────
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Println("  STEP 3 — Update title & item pertama via Forms BatchUpdate")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	requests := []*forms.Request{
		{
			UpdateFormInfo: &forms.UpdateFormInfoRequest{
				Info: &forms.Info{
					Title:       fmt.Sprintf("[MODIFIED] Form — %s", time.Now().Format("2006-01-02 15:04:05")),
					Description: "Form ini di-generate otomatis oleh go-doc-generator.",
				},
				UpdateMask: "title,description",
			},
		},
	}

	if len(copiedForm.Items) > 0 {
		firstItem := copiedForm.Items[0]
		updatedItem := &forms.Item{
			ItemId: firstItem.ItemId,
			Title:  "[DUMMY] Pertanyaan yang dimodifikasi",
		}
		if firstItem.QuestionItem != nil {
			updatedItem.QuestionItem = firstItem.QuestionItem
		}
		// ForceSendFields diperlukan agar Index=0 tidak dianggap "not set"
		// oleh proto3 (zero value problem pada integer field)
		loc := &forms.Location{Index: 0}
		loc.ForceSendFields = []string{"Index"}
		requests = append(requests, &forms.Request{
			UpdateItem: &forms.UpdateItemRequest{
				Item:       updatedItem,
				Location:   loc,
				UpdateMask: "title",
			},
		})
	}

	_, err = formsSvc.Forms.BatchUpdate(copiedForm.FormId, &forms.BatchUpdateFormRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌  Forms.BatchUpdate GAGAL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅  BatchUpdate berhasil! Title & item pertama dimodifikasi.")

	// ── STEP 4: Verifikasi hasil ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Println("  STEP 4 — Verifikasi hasil akhir")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	finalForm, err := formsSvc.Forms.Get(copiedForm.FormId).Context(ctx).Do()
	if err != nil {
		fmt.Printf("❌  Verifikasi gagal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅  Form final:\n")
	fmt.Printf("    Title        : %s\n", finalForm.Info.Title)
	fmt.Printf("    Description  : %s\n", finalForm.Info.Description)
	fmt.Printf("    Items        : %d buah\n", len(finalForm.Items))
	for i, item := range finalForm.Items {
		fmt.Printf("    [%d] %s\n", i, item.Title)
	}

	fmt.Println()
	fmt.Println("═════════════════════════════════════════════════════════════════")
	fmt.Println("  ✅  SEMUA STEP BERHASIL")
	fmt.Println()
	fmt.Printf("  Form ID      : %s\n", finalForm.FormId)
	fmt.Printf("  Responder URI: %s\n", finalForm.ResponderUri)
	fmt.Printf("  Drive Link   : %s\n", copiedFile.WebViewLink)
	fmt.Println("═════════════════════════════════════════════════════════════════")
}
