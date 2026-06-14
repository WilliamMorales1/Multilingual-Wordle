package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func dataPath(name string) string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, name)
}

func noCacheHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

func main() {
	initDB()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/languages", handleGetLanguages)
	mux.HandleFunc("GET /api/progress", handleGetProgress)
	mux.HandleFunc("POST /api/game", handleNewGame)
	mux.HandleFunc("GET /api/game/{id}", handleGetGame)
	mux.HandleFunc("POST /api/game/{id}/guess", handleGuess)
	mux.HandleFunc("GET /api/stats", handleGetStats)

	frontend := http.FileServer(http.Dir("../frontend"))
	mux.Handle("GET /frontend/", noCacheHandler(http.StripPrefix("/frontend/", frontend)))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../favicon.ico")
	})
	mux.HandleFunc("GET /manifest.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../frontend/manifest.json")
	})
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "../frontend/sw.js")
	})
	icons := http.FileServer(http.Dir("../frontend/icons"))
	mux.Handle("GET /icons/", http.StripPrefix("/icons/", icons))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "../frontend/index.html")
	})

	log.Println("Wordle server running → http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
