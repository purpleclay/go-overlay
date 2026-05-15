package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	slog.Info("ping-server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
