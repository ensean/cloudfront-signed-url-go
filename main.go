package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	signer, err := NewSigner(cfg)
	if err != nil {
		log.Fatalf("init signer: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sign", makeSignHandler(signer))
	mux.HandleFunc("/health", healthHandler)

	log.Printf("server listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// makeSignHandler returns an HTTP handler that produces a CloudFront signed URL.
//
// Query parameters:
//
//	path  (required) – S3 object path, e.g. /images/photo.jpg
//	ttl   (optional) – TTL in seconds; falls back to config default
func makeSignHandler(signer *Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "only GET is supported")
			return
		}

		s3Path := r.URL.Query().Get("path")
		if s3Path == "" {
			writeError(w, http.StatusBadRequest, "query parameter 'path' is required")
			return
		}

		var ttl time.Duration
		if ttlStr := r.URL.Query().Get("ttl"); ttlStr != "" {
			secs, err := strconv.Atoi(ttlStr)
			if err != nil || secs <= 0 {
				writeError(w, http.StatusBadRequest, "'ttl' must be a positive integer (seconds)")
				return
			}
			ttl = time.Duration(secs) * time.Second
		}

		signedURL, err := signer.Sign(s3Path, ttl)
		if err != nil {
			log.Printf("sign error for path=%q: %v", s3Path, err)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to sign URL: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"signed_url": signedURL,
			"path":       s3Path,
		})
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
