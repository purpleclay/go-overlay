package main

import (
	"encoding/json"
	"flag"
	"log"
	"math/rand/v2"
	"net/http"

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

	log.Printf("project-namer listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, r))
}
