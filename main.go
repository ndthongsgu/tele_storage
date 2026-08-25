package main

import (
	"log"
	"net/http"
	"path/filepath"

	"tele_storage/config"
	"tele_storage/handlers"
	"tele_storage/telegram"

	"github.com/go-chi/chi/v5"
)

func main() {
	log.Println("Starting Telegram API Microservice...")

	// 1. Load configuration
	cfg := config.LoadConfig()

	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		log.Println("⚠️  WARNING: TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is empty in .env!")
		log.Println("Please set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID in .env before sending requests.")
	}

	// 2. Initialize Telegram Client & Memory Store
	tgClient := telegram.NewClient(cfg.TelegramBotToken, cfg.TelegramChatID)
	store := telegram.NewStore()

	// Initial background sync from Telegram updates
	go store.SyncFromTelegram(tgClient)

	// 3. Create REST Server & Router
	srv := handlers.NewServer(cfg, tgClient, store)
	router := srv.Router().(*chi.Mux)

	// 4. Serve Interactive API Documentation UI
	staticDir := filepath.Join(".", "static")
	fs := http.FileServer(http.Dir(staticDir))

	router.Handle("/static/*", http.StripPrefix("/static/", fs))
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
	router.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	// 5. Start Server
	addr := ":" + cfg.Port
	log.Printf("🚀 Telegram API Service running at http://localhost%s\n", addr)
	log.Printf("📖 Interactive API Documentation UI accessible at http://localhost%s/\n", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server stopped unexpectedly: %v", err)
	}
}
