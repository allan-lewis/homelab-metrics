package main

import (
	"log"
	"net/http"

	"github.com/allan-lewis/homelab-metrics/internal/metrics"
	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()

	// Prometheus metrics endpoint
	r.Mount("/", metrics.Handler())

	log.Println("Starting homelab-metrics on :9102")

	if err := http.ListenAndServe(":9102", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
