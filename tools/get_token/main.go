// tools/get_token/main.go
//
// One-time setup: lakukan OAuth2 flow → simpan token ke token.json
//
// Setelah ini, backend bisa pakai token.json untuk akses Drive & Forms
// TANPA consent screen lagi — refresh_token bekerja secara silent.
//
// Usage (jalankan dari ROOT project):
//
//	go run ./tools/get_token/
//
// Token akan disimpan di: token.json (di root project)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	tokenFile = "token.json"
	oauthFile = "oauth-client-secret.json"
)

// Scope yang dibutuhkan untuk Drive + Forms
var scopes = []string{
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/drive.file",
	"https://www.googleapis.com/auth/forms.body",
	"https://www.googleapis.com/auth/forms.responses.readonly",
}

func main() {
	ctx := context.Background()

	// Baca OAuth client credentials dari oauth-client-secret.json
	b, err := os.ReadFile(oauthFile)
	if err != nil {
		log.Fatalf("❌  Gagal baca %s: %v", oauthFile, err)
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("❌  Gagal parse %s: %v", oauthFile, err)
	}
	config.RedirectURL = "http://localhost:9999/callback"

	// Generate authorization URL
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  Google OAuth2 Token Generator")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("  Membuka browser untuk authorization...")
	fmt.Printf("  URL: %s\n\n", authURL)

	// Coba buka browser otomatis
	openBrowser(authURL)

	// Tunggu callback dari browser
	codeCh := make(chan string, 1)
	srv := &http.Server{Addr: ":9999"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Authorization code tidak ditemukan", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "<html><body><h2>✅ Authorization berhasil!</h2><p>Silakan tutup tab ini.</p></body></html>")
		codeCh <- code
	})

	go func() {
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("Server error: %v", serveErr)
		}
	}()

	fmt.Println("  Menunggu authorization dari browser...")
	code := <-codeCh
	_ = srv.Shutdown(ctx)

	// Tukar code dengan token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		log.Fatalf("❌  Gagal exchange code: %v", err)
	}

	// Simpan token ke file
	f, err := os.Create(tokenFile)
	if err != nil {
		log.Fatalf("❌  Gagal buat token.json: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		log.Fatalf("❌  Gagal tulis token.json: %v", err)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  ✅  Token berhasil disimpan ke: %s\n", tokenFile)
	fmt.Println()
	fmt.Println("  Token ini mengandung refresh_token yang bisa dipakai")
	fmt.Println("  backend untuk akses Drive & Forms TANPA consent screen.")
	fmt.Println()
	fmt.Println("  Untuk production, simpan nilai-nilai ini di env vars:")
	fmt.Printf("  ACCESS_TOKEN  = %s\n", token.AccessToken[:min(20, len(token.AccessToken))]+"...")
	fmt.Printf("  REFRESH_TOKEN = %s\n", token.RefreshToken[:min(20, len(token.RefreshToken))]+"...")
	fmt.Printf("  TOKEN_TYPE    = %s\n", token.TokenType)
	fmt.Printf("  EXPIRY        = %s\n", token.Expiry.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("  Sekarang jalankan: go run ./tools/test_gform/")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	}
	args = append(args, url)
	_ = exec.Command(cmd, args...).Start()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
