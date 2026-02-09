package health

import (
	"net/http"
	"strconv"
)

// Handler returns an HTTP handler for health checks.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const body = `{"status":"ok"}`

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})
}
