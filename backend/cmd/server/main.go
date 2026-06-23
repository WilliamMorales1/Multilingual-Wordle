package main

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"wordgo/internal/api"
	"wordgo/internal/log"
	"wordgo/internal/store"
)

func noCacheHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

func main() {
	logger.Init()
	store.Init()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/languages", api.HandleGetLanguages)
	mux.HandleFunc("GET /api/progress", api.HandleGetProgress)
	mux.HandleFunc("POST /api/game", api.HandleNewGame)
	mux.HandleFunc("GET /api/game/{id}", api.HandleGetGame)
	mux.HandleFunc("POST /api/game/{id}/guess", api.HandleGuess)
	mux.HandleFunc("GET /api/stats", api.HandleGetStats)
	mux.HandleFunc("POST /api/cache/clear", api.HandleClearCache)
	mux.Handle("GET /metrics", promhttp.Handler())

	frontend := http.FileServer(http.Dir("../frontend/public"))
	mux.Handle("GET /frontend/", noCacheHandler(http.StripPrefix("/frontend/", frontend)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../frontend/public/favicon.ico")
	})
	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../frontend/public/manifest.json")
	})
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "../frontend/public/sw.js")
	})
	icons := http.FileServer(http.Dir("../frontend/public/icons"))
	mux.Handle("GET /icons/", http.StripPrefix("/icons/", icons))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "../frontend/public/index.html")
	})

	slog.Info("server starting", "addr", "http://localhost:8080")
	if err := http.ListenAndServe(":8080", logger.Middleware(mux)); err != nil {
		slog.Error("server failed", "err", err)
	}
}
