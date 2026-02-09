package main

import (
	"log"
	"net/http"
	"os"

	"github.com/allan-lewis/homelab-metrics/internal/metrics"
	"github.com/allan-lewis/homelab-metrics/internal/proxy"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	forwardURL := os.Getenv("FORWARD_METRICS_URL")
	if forwardURL != "" {
		log.Printf("Forwarding metrics requests to %s", forwardURL)
		r.Mount("/", proxy.Handler(forwardURL))
	} else {		
		log.Printf("Handling metrics requests directly")
		r.Mount("/", metrics.Handler())
	}

	// Default port
	port := os.Getenv("PORT")
	if port == "" {
		port = "9102"
	}

	addr := ":" + port

	log.Printf("Starting homelab-metrics on %s", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

}
