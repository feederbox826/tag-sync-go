package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"feederbox.cc/tag-sync-go/v2/internal"
)

func main() {
	cfg := internal.LoadConfig()
	if cfg.StashAPIKey == "" || cfg.StashURL == "" {
		log.Fatal("STASH_APIKEY and STASH_URL must be set")
	}

	fmt.Println("Starting tag sync")
	if _, err := internal.RunSync(cfg); err != nil {
		log.Fatalf("Initial sync failed: %v", err)
	}
	fmt.Println("Tag export complete", time.Now().UTC().Format(time.RFC3339))

	mux := http.NewServeMux()
	mux.HandleFunc("/update/await", handleUpdateAwait(cfg))
	mux.HandleFunc("/update", handleUpdate(cfg))

	go runCron(cfg)

	fmt.Printf("Listening on port %s\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}

func runCron(cfg *internal.Config) {
	for {
		now := time.Now()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		time.Sleep(time.Until(midnight))
		fmt.Println("Running scheduled tag sync")
		if _, err := internal.RunSync(cfg); err != nil {
			log.Printf("Cron sync failed: %v", err)
		}
	}
}

func handleUpdateAwait(cfg *internal.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result, err := internal.RunSync(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleUpdate(cfg *internal.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		go func() {
			if _, err := internal.RunSync(cfg); err != nil {
				log.Printf("Background sync failed: %v", err)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": time.Now().Format("15:04:05"),
		})
	}
}
