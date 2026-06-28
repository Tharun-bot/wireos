package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/Tharun-bot/wireos/backend/executor"
	"github.com/Tharun-bot/wireos/backend/intents"
	"github.com/Tharun-bot/wireos/backend/normalizer"
	"github.com/Tharun-bot/wireos/backend/wire"
)

var requestsTotal atomic.Int64

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		requestsTotal.Add(1)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode failed", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func intentsPath() string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "intents", "intents.yaml")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	})
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP wireos_requests_total Total HTTP requests handled\n")
	fmt.Fprintf(w, "# TYPE wireos_requests_total counter\n")
	fmt.Fprintf(w, "wireos_requests_total %d\n", requestsTotal.Load())
}

type intentRequest struct {
	IntentID string            `json:"intent_id"`
	Params   map[string]string `json:"params"`
}

type intentResponse struct {
	IntentID       string                        `json:"intent_id"`
	Label          string                        `json:"label"`
	Results        []normalizer.NormalizedResult `json:"results"`
	PartialFailure bool                          `json:"partial_failure"`
	TotalLatencyMs int64                         `json:"total_latency_ms"`
}

func makeIntentHandler(wireClient *wire.WireClient, allIntents []intents.Intent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req intentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.IntentID == "" {
			writeError(w, http.StatusBadRequest, "intent_id is required")
			return
		}

		intent, err := intents.FindIntent(allIntents, req.IntentID)
		if err != nil {
			writeError(w, http.StatusNotFound, fmt.Sprintf("unknown intent: %s", req.IntentID))
			return
		}

		slog.Info("intent request received",
			"intent_id", intent.ID,
			"action_count", len(intent.Actions),
		)

		ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()

		summary, err := executor.Execute(ctx, wireClient, intent, req.Params)
		if err != nil {
			slog.Error("executor failed", "intent_id", intent.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "execution failed: "+err.Error())
			return
		}

		normalized := make([]normalizer.NormalizedResult, 0, len(summary.Results))
		for _, execResult := range summary.Results {
			normalized = append(normalized, normalizer.Normalize(execResult))
		}

		slog.Info("intent executed",
			"intent_id", intent.ID,
			"label", intent.Label,
			"action_count", len(intent.Actions),
			"partial_failure", summary.PartialFailure,
			"total_latency_ms", summary.TotalLatencyMs,
			"total_credits", summary.TotalCredits,
		)

		writeJSON(w, http.StatusOK, intentResponse{
			IntentID:       intent.ID,
			Label:          intent.Label,
			Results:        normalized,
			PartialFailure: summary.PartialFailure,
			TotalLatencyMs: summary.TotalLatencyMs,
		})
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	apiKey := os.Getenv("ANAKIN_API_KEY")
	if apiKey == "" {
		slog.Error("ANAKIN_API_KEY environment variable is required")
		//os.Exit(1)
		apiKey = "ask_58024298a74a11f65ecf18d668537f03c29486fce50d8763704568acb6a55a6c"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	yamlPath := intentsPath()
	allIntents, err := intents.LoadIntents(yamlPath)
	if err != nil {
		slog.Error("failed to load intents", "path", yamlPath, "error", err)
		os.Exit(1)
	}
	slog.Info("intents loaded", "count", len(allIntents), "path", yamlPath)

	wireClient := wire.NewWireClient(apiKey)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /metrics", metricsHandler)
	mux.HandleFunc("/intent", makeIntentHandler(wireClient, allIntents))

	handler := withCORS(withLogging(mux))

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("wireos backend starting", "port", port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
