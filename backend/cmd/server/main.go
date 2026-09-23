package main

import (
	"log/slog"
	"net/http"

	"wordgo/internal/log"
	"wordgo/internal/server"
	"wordgo/internal/store"
)

func main() {
	logger.Init()
	store.Init()

	slog.Info("server starting", "addr", "http://localhost:8080")
	if err := http.ListenAndServe(server.DefaultAddr, server.Handler()); err != nil {
		slog.Error("server failed", "err", err)
	}
}
