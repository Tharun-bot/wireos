package normalizer

import (
	"testing"

	"github.com/Tharun-bot/wireos/backend/executor"
)

func makeResult(outputType, source string, data map[string]any) executor.ExecutorResult {
	return executor.ExecutorResult{
		IntentID:   "test",
		Source:     source,
		OutputType: outputType,
		Data:       data,
		LatencyMs:  42,
	}
}

// --- transaction ---

func TestNormalize_Transaction_StandardKeys(t *testing.T) {
	result := makeResult("transaction", "amazon.order_history", map[string]any{
		"orders": []any{
			map[string]any{
				"order_id":      "123",
				"merchant":      "Amazon",
				"total":         49.99,
				"currency":      "USD",
				"purchase_date": "2024-05-01",
				"department":    "Electronics",
			},
		},
	})

	norm := Normalize(result)
	if len(norm.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(norm.Transactions))
	}
	tx := norm.Transactions[0]
	if tx.ID != "123" {
		t.Errorf("ID = %q, want '123'", tx.ID)
	}
	if tx.Amount != 49.99 {
		t.Errorf("Amount = %v, want 49.99", tx.Amount)
	}
	if tx.RawSource != "amazon.order_history" {
		t.Errorf("RawSource = %q, want 'amazon.order_history'", tx.RawSource)
	}
	if tx.Category != "Electronics" {
		t.Errorf("Category = %q, want 'Electronics'", tx.Category)
	}
}

func TestNormalize_Transaction_NestedPrice(t *testing.T) {
	result := makeResult("transaction", "amazon.order_history", map[string]any{
		"items": []any{
			map[string]any{
				"id": "item-1",
				"price": map[string]any{
					"amount":   19.99,
					"currency": "GBP",
				},
			},
		},
	})

	norm := Normalize(result)
	if len(norm.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(norm.Transactions))
	}
	tx := norm.Transactions[0]
	if tx.Amount != 19.99 {
		t.Errorf("Amount = %v, want 19.99", tx.Amount)
	}
	if tx.Currency != "GBP" {
		t.Errorf("Currency = %q, want 'GBP'", tx.Currency)
	}
}

func TestNormalize_Transaction_DefaultCurrency(t *testing.T) {
	result := makeResult("transaction", "amazon.order_history", map[string]any{
		"orders": []any{
			map[string]any{"id": "x", "total": 5.0},
		},
	})
	norm := Normalize(result)
	if norm.Transactions[0].Currency != "USD" {
		t.Errorf("expected default currency USD, got %q", norm.Transactions[0].Currency)
	}
}

// --- activity ---

func TestNormalize_Activity_GitHubCommit(t *testing.T) {
	result := makeResult("activity", "github.user_activity", map[string]any{
		"commits": []any{
			map[string]any{
				"commit": map[string]any{
					"message": "fix: resolve race condition in executor",
				},
				"html_url":      "https://github.com/org/repo/commit/abc",
				"authored_date": "2024-05-10T12:00:00Z",
			},
		},
	})

	norm := Normalize(result)
	if len(norm.Activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(norm.Activities))
	}
	a := norm.Activities[0]
	if a.Summary != "fix: resolve race condition in executor" {
		t.Errorf("Summary = %q, unexpected", a.Summary)
	}
	if a.URL != "https://github.com/org/repo/commit/abc" {
		t.Errorf("URL = %q, unexpected", a.URL)
	}
	if a.Platform != "github" {
		t.Errorf("Platform = %q, want 'github'", a.Platform)
	}
}

func TestNormalize_Activity_DefaultType(t *testing.T) {
	result := makeResult("activity", "linkedin.activity_feed", map[string]any{
		"feed": []any{
			map[string]any{"text": "shared a post"},
		},
	})
	norm := Normalize(result)
	if norm.Activities[0].Type != "update" {
		t.Errorf("expected default type 'update', got %q", norm.Activities[0].Type)
	}
}

// --- contact ---

func TestNormalize_Contact_SplitName(t *testing.T) {
	result := makeResult("contact", "linkedin.connections", map[string]any{
		"connections": []any{
			map[string]any{
				"first_name":   "Priya",
				"last_name":    "Patel",
				"headline":     "SRE @ Google",
				"connected_at": "2024-04-20",
				"company": map[string]any{
					"name": "Google",
				},
			},
		},
	})

	norm := Normalize(result)
	if len(norm.Contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(norm.Contacts))
	}
	c := norm.Contacts[0]
	if c.Name != "Priya Patel" {
		t.Errorf("Name = %q, want 'Priya Patel'", c.Name)
	}
	if c.Role != "SRE @ Google" {
		t.Errorf("Role = %q, unexpected", c.Role)
	}
	if c.Company != "Google" {
		t.Errorf("Company = %q, want 'Google'", c.Company)
	}
	if c.Platform != "linkedin" {
		t.Errorf("Platform = %q, want 'linkedin'", c.Platform)
	}
}

// --- generic ---

func TestNormalize_Generic_PassThrough(t *testing.T) {
	data := map[string]any{"portfolio_value": 42000.0, "currency": "USD"}
	result := makeResult("generic", "robinhood.portfolio_summary", data)

	norm := Normalize(result)
	if norm.Generic == nil {
		t.Fatal("expected Generic to be set")
	}
	if norm.Generic["portfolio_value"] != 42000.0 {
		t.Errorf("Generic data not passed through correctly")
	}
	if len(norm.Transactions) != 0 || len(norm.Activities) != 0 {
		t.Error("generic result should not populate typed slices")
	}
}

// --- error passthrough ---

func TestNormalize_ErrorResult_NoDataParsed(t *testing.T) {
	result := executor.ExecutorResult{
		Source:     "amazon.order_history",
		OutputType: "transaction",
		Error:      "site returned 503",
		Data:       nil,
	}
	norm := Normalize(result)
	if norm.Error != "site returned 503" {
		t.Errorf("Error not propagated: %q", norm.Error)
	}
	if len(norm.Transactions) != 0 {
		t.Error("should not parse data when error is set")
	}
}

func TestNormalize_NilData_Safe(t *testing.T) {
	result := executor.ExecutorResult{
		Source:     "github.user_activity",
		OutputType: "activity",
		Data:       nil,
	}
	// Must not panic.
	norm := Normalize(result)
	if len(norm.Activities) != 0 {
		t.Error("nil data should produce empty activities")
	}
}

// --- helpers ---

func TestCoalesceStr_FirstMatch(t *testing.T) {
	m := map[string]any{"b": "second", "a": "first"}
	if got := coalesceStr(m, "a", "b"); got != "first" {
		t.Errorf("got %q, want 'first'", got)
	}
}

func TestCoalesceFloat_StringNumber(t *testing.T) {
	m := map[string]any{"amount": "29.95"}
	if got := coalesceFloat(m, "amount"); got != 29.95 {
		t.Errorf("got %v, want 29.95", got)
	}
}

func TestPlatformFromSource(t *testing.T) {
	cases := []struct{ in, want string }{
		{"amazon.order_history", "amazon"},
		{"github.commits", "github"},
		{"robinhood", "robinhood"},
		{"", ""},
	}
	for _, c := range cases {
		if got := platformFromSource(c.in); got != c.want {
			t.Errorf("platformFromSource(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
