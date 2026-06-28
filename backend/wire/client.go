package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://api.anakin.io"
	defaultTimeout = 10 * time.Second
	pollInterval   = 800 * time.Millisecond
	taskTimeout    = 30 * time.Second
)

type WireClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

type TaskRequest struct {
	ActionID string         `json:"action_id"`
	Params   map[string]any `json:"params,omitempty"`
	Identity string         `json:"identity,omitempty"`
}

type TaskResponse struct {
	JobID   string         `json:"job_id"`
	Status  string         `json:"status"`
	Result  map[string]any `json:"result,omitempty"`
	Credits int            `json:"credits,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type CatalogSite struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Actions int    `json:"actions"`
}

// --- internal API shapes ---

type taskSubmitResponse struct {
	JobID string `json:"job_id"`
}

type jobPollResponse struct {
	JobID   string         `json:"job_id"`
	Status  string         `json:"status"`
	Result  map[string]any `json:"result,omitempty"`
	Credits int            `json:"credits,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type identitiesResponse struct {
	Identities []string `json:"identities"`
}

// --- constructor ---

func NewWireClient(apiKey string) *WireClient {
	return &WireClient{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// --- core request helper ---

func (c *WireClient) do(ctx context.Context, method, url string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("wire: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("wire: build request: %w", err)
	}

	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wire: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("wire: read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wire: unexpected status %d: %s", resp.StatusCode, string(raw))
	}

	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("wire: decode response: %w", err)
		}
	}

	return nil
}

// --- RunTask: submit + poll ---

func (c *WireClient) RunTask(ctx context.Context, req TaskRequest) (*TaskResponse, error) {
	// Wrap the caller's ctx with a hard 30s ceiling for the whole operation.
	ctx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	// Step 1: submit the task.
	submitURL := c.baseURL + "/v1/wire/task"
	var submit taskSubmitResponse
	if err := c.do(ctx, http.MethodPost, submitURL, req, &submit); err != nil {
		return nil, fmt.Errorf("wire: submit task: %w", err)
	}
	if submit.JobID == "" {
		return nil, fmt.Errorf("wire: submit task: empty job_id in response")
	}

	// Step 2: poll until terminal state or context expires.
	pollURL := fmt.Sprintf("%s/v1/wire/jobs/%s", c.baseURL, submit.JobID)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wire: poll job %s: %w", submit.JobID, ctx.Err())
		case <-ticker.C:
			var poll jobPollResponse
			if err := c.do(ctx, http.MethodGet, pollURL, nil, &poll); err != nil {
				// Transient poll error — keep going unless ctx is done.
				continue
			}

			switch poll.Status {
			case "completed":
				return &TaskResponse{
					JobID:   poll.JobID,
					Status:  poll.Status,
					Result:  poll.Result,
					Credits: poll.Credits,
				}, nil
			case "failed":
				return &TaskResponse{
					JobID:  poll.JobID,
					Status: poll.Status,
					Error:  poll.Error,
				}, fmt.Errorf("wire: job %s failed: %s", poll.JobID, poll.Error)
			}
			// Any other status ("pending", "running") → keep polling.
		}
	}
}

// --- GetCatalog ---

func (c *WireClient) GetCatalog(ctx context.Context) ([]CatalogSite, error) {
	url := c.baseURL + "/v1/wire/catalog"
	var sites []CatalogSite
	if err := c.do(ctx, http.MethodGet, url, nil, &sites); err != nil {
		return nil, fmt.Errorf("wire: get catalog: %w", err)
	}
	if sites == nil {
		sites = []CatalogSite{}
	}
	return sites, nil
}

// --- ListIdentities ---

func (c *WireClient) ListIdentities(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/v1/wire/identities"

	// Try array shape first, fall back to wrapped object.
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodGet, url, nil, &raw); err != nil {
		return nil, fmt.Errorf("wire: list identities: %w", err)
	}

	// Shape 1: ["linkedin", "github", ...]
	var ids []string
	if err := json.Unmarshal(raw, &ids); err == nil {
		return ids, nil
	}

	// Shape 2: { "identities": [...] }
	var wrapped identitiesResponse
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("wire: list identities: decode: %w", err)
	}
	if wrapped.Identities == nil {
		return []string{}, nil
	}
	return wrapped.Identities, nil
}
