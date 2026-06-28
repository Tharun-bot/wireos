package executor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tharun-bot/wireos/backend/intents"
	"github.com/Tharun-bot/wireos/backend/wire"
)

// ---- mock Wire client ----

type mockWireClient struct {
	// fn is called per RunTask; tests set this to control behavior per action_id.
	fn func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error)
}

func (m *mockWireClient) RunTask(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
	return m.fn(ctx, req)
}

// successClient returns a successful response with the action_id echoed in result data.
func successClient(credits int) *mockWireClient {
	return &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			return &wire.TaskResponse{
				JobID:   "job-" + req.ActionID,
				Status:  "completed",
				Credits: credits,
				Result:  map[string]any{"action": req.ActionID, "ok": true},
			}, nil
		},
	}
}

// failingClient always returns an error.
func failingClient(msg string) *mockWireClient {
	return &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			return nil, errors.New(msg)
		},
	}
}

// ---- test intent builders ----

func makeIntent(id string, actions ...intents.Action) *intents.Intent {
	return &intents.Intent{
		ID:      id,
		Label:   id,
		Actions: actions,
	}
}

func staticAction(actionID, site, outputType string) intents.Action {
	return intents.Action{
		ActionID:         actionID,
		Site:             site,
		ParamsTemplate:   map[string]any{"limit": 10},
		IdentityRequired: false,
		OutputType:       outputType,
	}
}

// ---- tests ----

func TestExecute_AllSucceed_NoPartialFailure(t *testing.T) {
	intent := makeIntent("test_intent",
		staticAction("amazon.order_history", "amazon", "transaction"),
		staticAction("github.user_activity", "github", "activity"),
	)

	summary, err := Execute(context.Background(), successClient(3), intent, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if summary.PartialFailure {
		t.Error("expected PartialFailure=false when all actions succeed")
	}
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(summary.Results))
	}
	for _, r := range summary.Results {
		if r.Error != "" {
			t.Errorf("result for %s has unexpected error: %s", r.Source, r.Error)
		}
		if r.Data == nil {
			t.Errorf("result for %s has nil Data", r.Source)
		}
	}
	if summary.IntentID != "test_intent" {
		t.Errorf("IntentID = %q, want %q", summary.IntentID, "test_intent")
	}
}

func TestExecute_OneActionFails_PartialFailure(t *testing.T) {
	// Selective client: amazon fails, github succeeds.
	client := &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			if req.ActionID == "amazon.order_history" {
				return nil, errors.New("amazon: site returned 503")
			}
			return &wire.TaskResponse{
				Status:  "completed",
				Credits: 2,
				Result:  map[string]any{"data": "github stuff"},
			}, nil
		},
	}

	intent := makeIntent("mixed_intent",
		staticAction("amazon.order_history", "amazon", "transaction"),
		staticAction("github.user_activity", "github", "activity"),
	)

	summary, err := Execute(context.Background(), client, intent, nil)
	if err != nil {
		t.Fatalf("Execute should not return top-level error on partial failure, got: %v", err)
	}
	if !summary.PartialFailure {
		t.Error("expected PartialFailure=true when one action fails")
	}
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 results (one failure, one success), got %d", len(summary.Results))
	}

	var failCount, successCount int
	for _, r := range summary.Results {
		if r.Error != "" {
			failCount++
		} else {
			successCount++
		}
	}
	if failCount != 1 {
		t.Errorf("expected 1 failed result, got %d", failCount)
	}
	if successCount != 1 {
		t.Errorf("expected 1 successful result, got %d", successCount)
	}
}

func TestExecute_AllActionsFail_PartialFailure(t *testing.T) {
	intent := makeIntent("all_fail",
		staticAction("amazon.order_history", "amazon", "transaction"),
		staticAction("robinhood.portfolio", "robinhood", "generic"),
	)

	summary, err := Execute(context.Background(), failingClient("upstream error"), intent, nil)
	if err != nil {
		t.Fatalf("Execute should not return top-level error even when all actions fail: %v", err)
	}
	if !summary.PartialFailure {
		t.Error("expected PartialFailure=true when all actions fail")
	}
	if len(summary.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(summary.Results))
	}
}

func TestExecute_ContextCancelled_ReturnsError(t *testing.T) {
	// Client blocks until context is cancelled.
	client := &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	intent := makeIntent("slow_intent",
		staticAction("amazon.order_history", "amazon", "transaction"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// With a blocking client, Execute should eventually return because the
	// errgroup ctx is derived from our cancelled ctx.
	// Note: because we never return an error from the goroutine, Execute itself
	// won't error — but the RunTask calls will return ctx.Err(), which gets
	// encoded into ExecutorResult.Error.
	summary, err := Execute(ctx, client, intent, nil)

	// If ctx expired before goroutines finished, we get an error from eg.Wait.
	// If goroutines finished (with ctx.Err() encoded in result), we get PartialFailure.
	if err != nil {
		// This path: errgroup caught ctx cancellation before goroutines returned.
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			t.Errorf("expected context error, got: %v", err)
		}
		return
	}
	// This path: goroutines returned the ctx error as ExecutorResult.Error.
	if summary == nil {
		t.Fatal("expected summary, got nil")
	}
	if !summary.PartialFailure {
		t.Error("expected PartialFailure=true when context cancelled mid-execution")
	}
}

func TestExecute_ActionsRunConcurrently(t *testing.T) {
	// Each action sleeps 150ms. If they ran sequentially, total would be >300ms.
	// If concurrent, total should be ~150ms.
	const sleepDur = 150 * time.Millisecond

	var callCount atomic.Int32
	client := &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			callCount.Add(1)
			select {
			case <-time.After(sleepDur):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return &wire.TaskResponse{
				Status:  "completed",
				Credits: 1,
				Result:  map[string]any{},
			}, nil
		},
	}

	intent := makeIntent("concurrent_test",
		staticAction("action.one", "site1", "generic"),
		staticAction("action.two", "site2", "generic"),
		staticAction("action.three", "site3", "generic"),
	)

	start := time.Now()
	summary, err := Execute(context.Background(), client, intent, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(summary.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(summary.Results))
	}
	// Concurrent: should finish in ~sleepDur, not 3×sleepDur.
	if elapsed > 2*sleepDur {
		t.Errorf("actions do not appear to run concurrently: elapsed=%v (expected <%v)", elapsed, 2*sleepDur)
	}
}

func TestExecute_UserParamSubstitution(t *testing.T) {
	var capturedParams map[string]any

	client := &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			capturedParams = req.Params
			return &wire.TaskResponse{
				Status:  "completed",
				Credits: 1,
				Result:  map[string]any{},
			}, nil
		},
	}

	intent := makeIntent("param_test",
		intents.Action{
			ActionID: "github.user_activity",
			Site:     "github",
			ParamsTemplate: map[string]any{
				"username": "{user_input}",
				"days":     7,
			},
			IdentityRequired: false,
			OutputType:       "activity",
		},
	)

	_, err := Execute(context.Background(), client, intent, map[string]string{
		"username": "tharun-bot",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if capturedParams["username"] != "tharun-bot" {
		t.Errorf("username not substituted: got %v", capturedParams["username"])
	}
	if capturedParams["days"] != 7 {
		t.Errorf("static param 'days' should be 7, got %v", capturedParams["days"])
	}
}

func TestExecute_MissingUserParam_ResultHasError(t *testing.T) {
	intent := makeIntent("missing_param",
		intents.Action{
			ActionID:         "github.user_activity",
			Site:             "github",
			ParamsTemplate:   map[string]any{"username": "{user_input}"},
			IdentityRequired: false,
			OutputType:       "activity",
		},
	)

	// Pass empty userParams — username is required but missing.
	summary, err := Execute(context.Background(), successClient(1), intent, map[string]string{})
	if err != nil {
		t.Fatalf("Execute should not error at top level: %v", err)
	}
	if !summary.PartialFailure {
		t.Error("expected PartialFailure=true for missing user param")
	}
	if summary.Results[0].Error == "" {
		t.Error("expected error in result for missing param")
	}
}

func TestExecute_IdentityRequired_SetsIdentityField(t *testing.T) {
	var capturedIdentity string

	client := &mockWireClient{
		fn: func(ctx context.Context, req wire.TaskRequest) (*wire.TaskResponse, error) {
			capturedIdentity = req.Identity
			return &wire.TaskResponse{
				Status:  "completed",
				Credits: 1,
				Result:  map[string]any{},
			}, nil
		},
	}

	intent := makeIntent("identity_test",
		intents.Action{
			ActionID:         "amazon.order_history",
			Site:             "amazon",
			ParamsTemplate:   map[string]any{},
			IdentityRequired: true,
			OutputType:       "transaction",
		},
	)

	_, err := Execute(context.Background(), client, intent, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if capturedIdentity != "amazon" {
		t.Errorf("expected Identity='amazon', got %q", capturedIdentity)
	}
}

func TestExecute_NilIntent_ReturnsError(t *testing.T) {
	_, err := Execute(context.Background(), successClient(1), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil intent, got nil")
	}
}

// ---- resolveParams unit tests ----

func TestResolveParams_StaticValues(t *testing.T) {
	template := map[string]any{
		"days":  30,
		"limit": 50,
		"flag":  true,
	}
	out, err := resolveParams(template, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["days"] != 30 || out["limit"] != 50 || out["flag"] != true {
		t.Errorf("static values not preserved: %v", out)
	}
}

func TestResolveParams_UserInputSubstituted(t *testing.T) {
	template := map[string]any{"query": "{user_input}"}
	out, err := resolveParams(template, map[string]string{"query": "my search"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["query"] != "my search" {
		t.Errorf("expected 'my search', got %v", out["query"])
	}
}

func TestResolveParams_MissingUserInput_ReturnsError(t *testing.T) {
	template := map[string]any{"username": "{user_input}"}
	_, err := resolveParams(template, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing user_input key")
	}
}

func TestResolveParams_EmptyTemplate(t *testing.T) {
	out, err := resolveParams(map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map, got %v", out)
	}
}
