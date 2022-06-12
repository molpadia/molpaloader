package main

import (
	"encoding/json"
	"net/http"
	"os"
)

const (
	MaxRequestSize = 1 << 20
)

// Get the value of environment variables.
func env(key string, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

// Parse incoming request body as JSON object.
func parse(w http.ResponseWriter, r *http.Request, data interface{}) (err error) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(MaxRequestSize))
	dec := json.NewDecoder(r.Body)

	if err = dec.Decode(data); err != nil {
		return err
	}

	return
}

// Response the output with JSON format.
func response(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		return err
	}
	return nil
}
