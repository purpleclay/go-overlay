package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/purpleclay/go-overlay/examples/oapi-codegen/gen"
)

//go:embed static
var staticFiles embed.FS

//go:embed index.html
var indexHTML []byte

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	flag.Parse()

	mux := http.NewServeMux()
	gen.HandlerFromMux(gen.NewStrictHandler(&cattoServer{}, nil), mux)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML) //nolint:errcheck
	})

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /fragment", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			return
		}
		idx := countVowels(name) % len(breeds)
		b := breeds[idx]
		img := breedImages[idx]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w,
			`<div class="text-center"><img src="/static/%s" alt="%s" class="[image-rendering:pixelated] w-full aspect-square object-contain mb-5"><h2 class="text-crimson text-xl font-bold uppercase tracking-widest mb-2">%s</h2><p class="text-muted text-sm italic">%s</p></div>`,
			img, b.Breed, b.Breed, b.Reason,
		)
	})

	log.Printf("catto listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
