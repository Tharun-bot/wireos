package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Tharun-bot/wireos/backend/intents"
	"github.com/Tharun-bot/wireos/backend/wire"
)

// WireClientInterface allows mocking in tests.
type WireClientInterface interface {
	RunTask(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error)
}

// ExecutorResult holds the outcome of one Wire action.
type ExecutorResult struct {
	IntentID   string         `json:"intent_id"`
	Source     string         `json:"source"` // action_id, e.g. "amazon.order_history"
	OutputType string         `json:"output_type"`
	Data       map[string]any `json:"data,omitempty"`
	Error      string         `json:"error,omitempty"`
	LatencyMs  int64          `json:"latency_ms"`
}

// ExecutionSummary is the merged output of all actions for one intent.
type ExecutionSummary struct {
	IntentID       string           `json:"intent_id"`
	Results        []ExecutorResult `json:"results"`
	TotalCredits   int              `json:"total_credits"`
	TotalLatencyMs int64            `json:"total_latency_ms"`
	PartialFailure bool             `json:"partial_failure"`
}

// Execute fans out all actions in the intent in parallel and merges results.
// A single action failure sets PartialFailure=true but never aborts the summary.
// The caller's ctx deadline governs the entire operation.
func Execute(
	ctx context.Context,
	wireClient WireClientInterface,
	intent *intents.Intent,
	userParams map[string]string,
) (*ExecutionSummary, error) {
	if intent == nil {
		return nil, fmt.Errorf("executor: intent is nil")
	}

	start := time.Now()

	// results channel — buffered so goroutines never block on send.
	resultsCh := make(chan ExecutorResult, len(intent.Actions))

	// errgroup gives us structured goroutine lifecycle + ctx cancellation.
	// We use the base ctx directly: if it's cancelled, all child RunTask calls
	// see the cancellation immediately.
	eg, egCtx := errgroup.WithContext(ctx)

	for _, action := range intent.Actions {
		action := action // capture loop variable

		eg.Go(func() error {
			result := runAction(egCtx, wireClient, intent.ID, action, userParams)
			resultsCh <- result
			// Never return a non-nil error here: we handle per-action failures
			// via PartialFailure, not by aborting the errgroup.
			return nil
		})
	}

	// Wait for all goroutines. Since no goroutine ever returns an error,
	// eg.Wait() only fails if the errgroup's own context is cancelled — which
	// happens when the caller's ctx is cancelled.
	if err := eg.Wait(); err != nil {
		// Context cancelled before all goroutines finished.
		return nil, fmt.Errorf("executor: %w", err)
	}
	close(resultsCh)

	summary := &ExecutionSummary{
		IntentID:       intent.ID,
		Results:        make([]ExecutorResult, 0, len(intent.Actions)),
		TotalLatencyMs: time.Since(start).Milliseconds(),
	}

	for r := range resultsCh {
		summary.Results = append(summary.Results, r)
		if r.Error != "" {
			summary.PartialFailure = true
		}
		// Credits are only present on success; zero on failure — safe to sum always.
	}

	// Sum credits from the wire responses (stored in Data["_credits"] by runAction).
	for _, r := range summary.Results {
		if r.Data != nil {
			if c, ok := r.Data["_credits"].(int); ok {
				summary.TotalCredits += c
				delete(r.Data, "_credits") // clean internal key before returning
			}
		}
	}

	return summary, nil
}

// runAction executes a single Wire action and returns a fully-populated ExecutorResult.
// It never panics and always returns — errors are encoded into ExecutorResult.Error.
func runAction(
	ctx context.Context,
	client WireClientInterface,
	intentID string,
	action intents.Action,
	userParams map[string]string,
) ExecutorResult {
	start := time.Now()

	base := ExecutorResult{
		IntentID:   intentID,
		Source:     action.ActionID,
		OutputType: action.OutputType,
	}

	params, err := resolveParams(action.ParamsTemplate, userParams)
	if err != nil {
		base.Error = fmt.Sprintf("param resolution failed: %v", err)
		base.LatencyMs = time.Since(start).Milliseconds()
		return base
	}

	req := wire.TaskRequest{
		ActionID: action.ActionID,
		Params:   params,
	}
	// Identity field on TaskRequest is the site slug when required.
	// The Wire API uses the stored OAuth token for that identity at runtime.
	if action.IdentityRequired {
		req.Identity = action.Site
	}

	resp, err := client.RunTask(ctx, req)
	base.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		base.Error = err.Error()
		return base
	}
	if resp == nil {
		base.Error = "nil response from wire client"
		return base
	}
	if resp.Error != "" {
		base.Error = resp.Error
		return base
	}

	// Stash credits in Data under an internal key so Execute() can sum them.
	data := resp.Result
	if data == nil {
		data = make(map[string]any)
	}
	data["_credits"] = resp.Credits
	base.Data = data

	return base
}

// resolveParams substitutes "{user_input}" placeholders in a params template.
// Static values (int, bool, etc.) are passed through unchanged.
// Returns an error if a required user_input key is missing.
func resolveParams(template map[string]any, userParams map[string]string) (map[string]any, error) {
	resolved := make(map[string]any, len(template))
	var missing []string

	for k, v := range template {
		str, isString := v.(string)
		if isString && str == "{user_input}" {
			val, ok := userParams[k]
			if !ok || val == "" {
				missing = append(missing, k)
				continue
			}
			resolved[k] = val
		} else {
			resolved[k] = v
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing user_input values for params: %v", missing)
	}

	return resolved, nil
}

// MustResolveParams is resolveParams that panics — useful in tests building fixtures.
func MustResolveParams(template map[string]any, userParams map[string]string) map[string]any {
	out, err := resolveParams(template, userParams)
	if err != nil {
		panic(err)
	}
	return out
}

// concurrentCollect drains a channel into a slice under a mutex.
// Exported for use in integration tests that build their own channels.
func concurrentCollect(ch <-chan ExecutorResult) []ExecutorResult {
	var mu sync.Mutex
	var out []ExecutorResult
	for r := range ch {
		mu.Lock()
		out = append(out, r)
		mu.Unlock()
	}
	return out
}
