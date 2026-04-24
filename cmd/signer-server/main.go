package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/GoPolymarket/polymarket-go-sdk/pkg/auth"
)

// Request payload matching what the SDK sends
type SignRequest struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

// Legacy V1 builder HMAC header names. These are deprecated in V2 (which uses
// the builderCode field on orders instead) but kept for backward compatibility.
const (
	headerBuilderAPIKey     = "POLY_BUILDER_API_KEY"
	headerBuilderPassphrase = "POLY_BUILDER_PASSPHRASE"
	headerBuilderSignature  = "POLY_BUILDER_SIGNATURE"
	headerBuilderTimestamp  = "POLY_BUILDER_TIMESTAMP"
)

// Env vars
var (
	BuilderKey        = os.Getenv("BUILDER_KEY")
	BuilderSecret     = os.Getenv("BUILDER_SECRET")
	BuilderPassphrase = os.Getenv("BUILDER_PASSPHRASE")
)

func main() {
	if BuilderKey == "" || BuilderSecret == "" || BuilderPassphrase == "" {
		log.Fatal("Missing BUILDER_KEY, BUILDER_SECRET, or BUILDER_PASSPHRASE env vars")
	}

	http.HandleFunc("/v1/sign-builder", handleSign)
	http.HandleFunc("/health", handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Signer service running on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func handleSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	// Logic from auth.BuildL2Headers but simplified for Builder
	// Message = timestamp + method + path + body
	message := fmt.Sprintf("%d%s%s", req.Timestamp, req.Method, req.Path)
	if req.Body != "" {
		message += req.Body
	}

	// Sign using the Secret (held securely on this server)
	sig, err := auth.SignHMAC(BuilderSecret, message)
	if err != nil {
		log.Printf("Signing error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// NOTE: Builder HMAC headers are deprecated in CLOB API V2.
	// V2 uses the `builderCode` field on orders instead.
	// This endpoint is kept for backward compatibility with V1 infrastructure only.
	resp := map[string]string{
		headerBuilderAPIKey:     BuilderKey,
		headerBuilderPassphrase: BuilderPassphrase,
		headerBuilderTimestamp:  fmt.Sprintf("%d", req.Timestamp),
		headerBuilderSignature:  sig,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
