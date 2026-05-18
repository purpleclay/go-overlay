package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
)

var (
	adjectives = []string{
		"caffeinated", "chaotic", "cursed", "feral", "grumpy",
		"haunted", "overzealous", "panicking", "rogue", "sleep-deprived",
		"sneaky", "spicy", "suspicious", "unhinged", "wobbly",
		"yolo", "zealous", "frantic", "deranged", "stubborn",
	}

	nouns = []string{
		"badger", "ferret", "goblin", "gremlin", "hamster",
		"kraken", "lemur", "narwhal", "penguin", "platypus",
		"raccoon", "walrus", "wizard", "yak", "ninja",
		"gopher", "bandit", "gremlin", "dragon", "wombat",
	}
)

func randomName() string {
	adj := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]
	return adj + "-" + noun
}

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	r := chi.NewRouter()

	r.Get("/name", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": randomName()})
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("project-namer listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}
