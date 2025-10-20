package api

import "net/http"

// HandleDocs handles GET /docs
// Redirects to the canonical documentation at muxi.org
func (s *Server) HandleDocs(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://muxi.org/docs", http.StatusFound)
}
