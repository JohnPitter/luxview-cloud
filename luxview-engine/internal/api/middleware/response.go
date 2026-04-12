package middleware

import (
	"encoding/json"
	"net/http"
)

// writeJSONError writes a JSON error response with the correct Content-Type header.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
