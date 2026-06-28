package normalizer

import (
	"fmt"
	"strings"

	"github.com/Tharun-bot/wireos/backend/executor"
)

// --- Canonical output types ---

type Transaction struct {
	ID        string  `json:"id"`
	Merchant  string  `json:"merchant"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Date      string  `json:"date"`
	Category  string  `json:"category"`
	RawSource string  `json:"raw_source"`
}

type Activity struct {
	Platform  string `json:"platform"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Summary   string `json:"summary"`
	URL       string `json:"url"`
	RawSource string `json:"raw_source"`
}

type Contact struct {
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	Role            string `json:"role"`
	Company         string `json:"company"`
	LastInteraction string `json:"last_interaction"`
	RawSource       string `json:"raw_source"`
}

type NormalizedResult struct {
	OutputType   string         `json:"output_type"`
	Source       string         `json:"source"`
	Error        string         `json:"error,omitempty"`
	LatencyMs    int64          `json:"latency_ms"`
	Transactions []Transaction  `json:"transactions,omitempty"`
	Activities   []Activity     `json:"activities,omitempty"`
	Contacts     []Contact      `json:"contacts,omitempty"`
	Generic      map[string]any `json:"generic,omitempty"`
}

// Normalize converts a raw ExecutorResult into a typed NormalizedResult.
// It never panics — all map accesses are guarded with ok checks.
func Normalize(result executor.ExecutorResult) NormalizedResult {
	out := NormalizedResult{
		OutputType: result.OutputType,
		Source:     result.Source,
		Error:      result.Error,
		LatencyMs:  result.LatencyMs,
	}

	// Propagate errors without attempting to parse nil/empty data.
	if result.Error != "" || result.Data == nil {
		return out
	}

	switch result.OutputType {
	case "transaction":
		out.Transactions = normalizeTransactions(result.Data, result.Source)
	case "activity":
		out.Activities = normalizeActivities(result.Data, result.Source)
	case "contact":
		out.Contacts = normalizeContacts(result.Data, result.Source)
	default:
		// "generic" and any unknown types pass through as-is.
		out.Generic = result.Data
	}

	return out
}

// --- transaction normalizer ---

// candidateListKeys lists the common envelope keys Wire sites use for order/transaction arrays.
var transactionListKeys = []string{
	"orders", "transactions", "items", "purchases",
	"order_history", "data", "results",
}

func normalizeTransactions(data map[string]any, source string) []Transaction {
	items := extractList(data, transactionListKeys)
	if items == nil {
		// Treat the whole object as a single transaction record.
		items = []any{data}
	}

	out := make([]Transaction, 0, len(items))
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t := Transaction{
			ID:        coalesceStr(m, "id", "order_id", "transaction_id", "asin"),
			Merchant:  coalesceStr(m, "merchant", "seller", "store", "retailer", "brand"),
			Amount:    coalesceFloat(m, "amount", "total", "price", "grand_total", "order_total"),
			Currency:  coalesceStr(m, "currency", "currency_code"),
			Date:      coalesceStr(m, "date", "ordered_date", "purchase_date", "created_at", "timestamp"),
			Category:  coalesceStr(m, "category", "department", "product_type", "type"),
			RawSource: source,
		}
		if t.Currency == "" {
			t.Currency = "USD" // sensible default for US-centric Wire sources
		}
		// Flatten nested price objects: {"price": {"amount": 9.99, "currency": "USD"}}
		if t.Amount == 0 {
			if priceObj, ok := m["price"].(map[string]any); ok {
				t.Amount = coalesceFloat(priceObj, "amount", "value")
				if t.Currency == "USD" {
					if c := coalesceStr(priceObj, "currency", "currency_code"); c != "" {
						t.Currency = c
					}
				}
			}
		}
		out = append(out, t)
	}
	return out
}

// --- activity normalizer ---

var activityListKeys = []string{
	"activities", "events", "feed", "items",
	"commits", "posts", "updates", "data", "results",
}

func normalizeActivities(data map[string]any, source string) []Activity {
	items := extractList(data, activityListKeys)
	if items == nil {
		items = []any{data}
	}

	platform := platformFromSource(source)

	out := make([]Activity, 0, len(items))
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		a := Activity{
			Platform:  platform,
			Type:      coalesceStr(m, "type", "event_type", "activity_type", "action"),
			Timestamp: coalesceStr(m, "timestamp", "created_at", "date", "published_at", "authored_date"),
			Summary:   coalesceStr(m, "summary", "message", "text", "title", "description", "body", "content"),
			URL:       coalesceStr(m, "url", "html_url", "link", "permalink"),
			RawSource: source,
		}
		// GitHub commits: summary lives under commit.message
		if a.Summary == "" {
			if commitObj, ok := m["commit"].(map[string]any); ok {
				a.Summary = coalesceStr(commitObj, "message", "summary")
			}
		}
		// LinkedIn: actor name as summary fallback
		if a.Summary == "" {
			a.Summary = coalesceStr(m, "actor", "author", "name")
		}
		if a.Type == "" {
			a.Type = "update"
		}
		out = append(out, a)
	}
	return out
}

// --- contact normalizer ---

var contactListKeys = []string{
	"contacts", "connections", "people", "members",
	"results", "data", "items",
}

func normalizeContacts(data map[string]any, source string) []Contact {
	items := extractList(data, contactListKeys)
	if items == nil {
		items = []any{data}
	}

	platform := platformFromSource(source)

	out := make([]Contact, 0, len(items))
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		// Name may be split across first/last fields.
		name := coalesceStr(m, "name", "full_name", "display_name")
		if name == "" {
			first := coalesceStr(m, "first_name", "firstName")
			last := coalesceStr(m, "last_name", "lastName")
			name = strings.TrimSpace(first + " " + last)
		}

		c := Contact{
			Name:            name,
			Platform:        platform,
			Role:            coalesceStr(m, "role", "title", "headline", "position", "job_title"),
			Company:         coalesceStr(m, "company", "organization", "employer", "company_name"),
			LastInteraction: coalesceStr(m, "last_interaction", "last_seen", "connected_at", "updated_at"),
			RawSource:       source,
		}
		// LinkedIn nested company object: {"company": {"name": "Acme"}}
		if c.Company == "" {
			if compObj, ok := m["company"].(map[string]any); ok {
				c.Company = coalesceStr(compObj, "name", "display_name")
			}
		}
		out = append(out, c)
	}
	return out
}

// --- safe map helpers ---

// extractList looks for the first matching key in data that holds a []any.
func extractList(data map[string]any, keys []string) []any {
	for _, k := range keys {
		v, ok := data[k]
		if !ok {
			continue
		}
		if list, ok := v.([]any); ok {
			return list
		}
	}
	return nil
}

// coalesceStr returns the string value of the first key found in m.
func coalesceStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch s := v.(type) {
		case string:
			if s != "" {
				return s
			}
		case fmt.Stringer:
			if str := s.String(); str != "" {
				return str
			}
		}
	}
	return ""
}

// coalesceFloat returns the float64 value of the first key found in m.
// Handles JSON numbers (float64) and strings that look like numbers.
func coalesceFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			if n != 0 {
				return n
			}
		case int:
			return float64(n)
		case int64:
			return float64(n)
		case string:
			var f float64
			if _, err := fmt.Sscanf(n, "%f", &f); err == nil && f != 0 {
				return f
			}
		}
	}
	return 0
}

// platformFromSource extracts the site name from "site.action_name" action IDs.
func platformFromSource(source string) string {
	parts := strings.SplitN(source, ".", 2)
	if len(parts) == 0 || parts[0] == "" {
		return source
	}
	return parts[0]
}
