// Command golem-api runs the GOLEM API edge.
//
// Skeleton for Fase 0 (bootstrap): it only exposes liveness. The first
// vertical slice (M1, weeks 7–8) wires commands through the application
// layer to the Graph Journal and graph projection.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("GOLEM_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			log.Printf("healthz encode: %v", err)
		}
	})

	log.Printf("golem-api listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
