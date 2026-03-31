package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/purpleclay/go-overlay/examples/go-workspace/mood"
)

//go:embed templates/*
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.html"))

func today() string {
	return time.Now().Format("2006-01-02")
}

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/rate", handleRate)

	log.Printf("mood check-in listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	moodsJSON, _ := json.Marshal(mood.All())

	data := map[string]any{
		"AlreadyCheckedIn": false,
		"BgColor":          "#6b7280",
		"MoodsJSON":        template.JS(moodsJSON),
	}

	if c, err := r.Cookie("checkin"); err == nil {
		parts := strings.SplitN(c.Value, ":", 2)
		if len(parts) == 2 && parts[1] == today() {
			score, _ := strconv.Atoi(parts[0])
			feeling := mood.For(score)
			data["AlreadyCheckedIn"] = true
			data["Score"] = score
			data["Name"] = feeling.Label
			data["Anecdote"] = feeling.Anecdote
			data["BgColor"] = feeling.Color
		}
	}

	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleRate(w http.ResponseWriter, r *http.Request) {
	score, err := strconv.Atoi(r.URL.Query().Get("score"))
	if err != nil || score < 1 {
		score = 1
	}
	if score > 11 {
		score = 11
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "checkin",
		Value:   fmt.Sprintf("%d:%s", score, today()),
		Path:    "/",
		Expires: time.Now().Add(24 * time.Hour),
	})

	// Trigger a full page reload so handleIndex renders the checked-in state.
	w.Header().Set("HX-Refresh", "true")
}
