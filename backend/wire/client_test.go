package wire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestClient wires up a WireClient pointing at a fake server.
func newTestClient(server *httptest.Server) *WireClient {
	return &WireClient{
		apiKey:  "test-key",
		baseURL: server.URL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// mustJSON marshals v or panics — test helper only.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- RunTask tests ----

func TestRunTask_Success(t *testing.T) {
	// The fake server handles two routes:
	//   POST /v1/wire/task  → returns a job_id
	//   GET  /v1/wire/jobs/:id → first call returns "running", second "completed"
	pollCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Auth header must be present.
		if r.Header.Get("X-API-Key") != "test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/wire/task":
			w.Header().Set("Content-Type", "application/json")
			w.Write(mustJSON(taskSubmitResponse{JobID: "job-123"}))

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/wire/jobs/"):
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			if pollCount < 2 {
				// First poll: still running.
				w.Write(mustJSON(jobPollResponse{JobID: "job-123", Status: "running"}))
			} else {
				// Second poll: done.
				w.Write(mustJSON(jobPollResponse{
					JobID:   "job-123",
					Status:  "completed",
					Result:  map[string]any{"answer": 42},
					Credits: 5,
				}))
			}

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	// Use a short poll interval so the test doesn't wait 800ms.
	// We swap it out inline since it's a package-level const — override via a
	// short context deadline that is still long enough to succeed.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.RunTask(ctx, TaskRequest{
		ActionID: "amazon.order_history",
		Params:   map[string]any{"days": 30},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", resp.Status)
	}
	if resp.JobID != "job-123" {
		t.Errorf("expected job_id 'job-123', got %q", resp.JobID)
	}
	if resp.Credits != 5 {
		t.Errorf("expected 5 credits, got %d", resp.Credits)
	}
	if v, ok := resp.Result["answer"]; !ok || v != float64(42) {
		t.Errorf("unexpected result: %v", resp.Result)
	}
}

func TestRunTask_Timeout(t *testing.T) {
	// Server always returns "pending" — RunTask should time out.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/wire/task":
			w.Header().Set("Content-Type", "application/json")
			w.Write(mustJSON(taskSubmitResponse{JobID: "job-stuck"}))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			w.Write(mustJSON(jobPollResponse{JobID: "job-stuck", Status: "pending"}))
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	// Caller gives a 1-second deadline; RunTask's own 30s ceiling is irrelevant here.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := client.RunTask(ctx, TaskRequest{ActionID: "some.action"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "wire:") {
		t.Errorf("error should be wrapped with 'wire:' prefix, got: %v", err)
	}
}

func TestRunTask_JobFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.Write(mustJSON(taskSubmitResponse{JobID: "job-fail"}))
		case r.Method == http.MethodGet:
			w.Write(mustJSON(jobPollResponse{
				JobID:  "job-fail",
				Status: "failed",
				Error:  "site returned 403",
			}))
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	resp, err := client.RunTask(ctx, TaskRequest{ActionID: "bad.action"})
	if err == nil {
		t.Fatal("expected error for failed job, got nil")
	}
	if resp == nil || resp.Status != "failed" {
		t.Errorf("expected failed TaskResponse, got: %+v", resp)
	}
	if !strings.Contains(err.Error(), "site returned 403") {
		t.Errorf("error should contain job error message, got: %v", err)
	}
}

// ---- GetCatalog tests ----

func TestGetCatalog_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/wire/catalog" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := newTestClient(server)
	sites, err := client.GetCatalog(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("expected empty slice, got %d items", len(sites))
	}
}

func TestGetCatalog_WithData(t *testing.T) {
	expected := []CatalogSite{
		{Slug: "amazon", Name: "Amazon", Actions: 4},
		{Slug: "github", Name: "GitHub", Actions: 7},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(expected))
	}))
	defer server.Close()

	client := newTestClient(server)
	sites, err := client.GetCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
	if sites[0].Slug != "amazon" || sites[1].Slug != "github" {
		t.Errorf("unexpected sites: %+v", sites)
	}
}

// ---- ListIdentities tests ----

func TestListIdentities_ArrayShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON([]string{"linkedin", "github", "amazon"}))
	}))
	defer server.Close()

	client := newTestClient(server)
	ids, err := client.ListIdentities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 identities, got %d", len(ids))
	}
}

func TestListIdentities_WrappedShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mustJSON(identitiesResponse{Identities: []string{"robinhood"}}))
	}))
	defer server.Close()

	client := newTestClient(server)
	ids, err := client.ListIdentities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "robinhood" {
		t.Errorf("unexpected identities: %v", ids)
	}
}

// ---- Auth header test (cross-cutting) ----

func TestAPIKeyHeader_IsSentOnAllRequests(t *testing.T) {
	tests := []struct {
		name    string
		call    func(c *WireClient, s *httptest.Server) error
		path    string
		method  string
		handler http.HandlerFunc
	}{
		{
			name:   "GetCatalog sends X-API-Key",
			method: http.MethodGet,
			path:   "/v1/wire/catalog",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("[]"))
			},
			call: func(c *WireClient, s *httptest.Server) error {
				_, err := c.GetCatalog(context.Background())
				return err
			},
		},
		{
			name:   "ListIdentities sends X-API-Key",
			method: http.MethodGet,
			path:   "/v1/wire/identities",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write(mustJSON([]string{}))
			},
			call: func(c *WireClient, s *httptest.Server) error {
				_, err := c.ListIdentities(context.Background())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotKey string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotKey = r.Header.Get("X-API-Key")
				tt.handler(w, r)
			}))
			defer server.Close()

			client := newTestClient(server)
			if err := tt.call(client, server); err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if gotKey != "test-key" {
				t.Errorf("expected X-API-Key 'test-key', got %q", gotKey)
			}
		})
	}
}
