package api

import "net/http"

// HandlePing handles GET /ping
// Returns "pong" for simple connectivity testing
func (s *Server) HandlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}
