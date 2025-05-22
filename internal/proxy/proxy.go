package proxy

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler returns an http.Handler that proxies requests to the given targetURL
func Handler(targetURL string) http.Handler {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("Invalid FORWARD_METRICS_URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Override the Director to fix path to /metrics
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = "/metrics"
		req.Host = target.Host
	}

	return proxy
}
