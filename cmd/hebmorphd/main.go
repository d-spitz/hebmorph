// Command hebmorphd serves the hebmorph morphological analyzer over HTTP.
//
//	hebmorphd -addr :8080
//	curl -sG --data-urlencode 'w=מלכה' http://localhost:8080/api/v1/analyze/$w
//
// The API is described by api/openapi.yaml, also served at
// /api/v1/openapi.yaml.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"hebmorph"
	"hebmorph/api"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	a, err := hebmorph.New()
	if err != nil {
		log.Fatalln("hebmorphd:", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.NewHandler(a),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	log.Println("hebmorphd listening on", *addr)
	log.Fatalln("hebmorphd:", srv.ListenAndServe())
}
