// Command fakebgg serves deterministic BoardGameGeek XML API2 responses.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	fakebgg "boardgames/internal/bgg/fake"
)

func main() {
	addr := flag.String("addr", ":6291", "listen address")
	flag.Parse()
	server := &http.Server{Addr: *addr, Handler: fakebgg.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("fake BGG listening on http://localhost%s/xmlapi2", *addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
