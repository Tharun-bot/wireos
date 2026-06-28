package intents

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

const minimalYAML = `
intents:
  - id: recent_purchases
    label: "Recent Purchases"
    description: "What did I buy recently?"
    actions:
      - action_id: amazon.order_history
        site: amazon
        params_template:
          days: 30
        identity_required: true
        output_type: transaction

  - id: github_activity
    label: "GitHub Activity"
    description: "Show my recent GitHub work."
    actions:
      - action_id: github.user_activity
        site: github
        params_template:
          username: "{user_input}"
          days: 7
        identity_required: false
        output_type: activity
`

func TestLoadIntents_ParsesCorrectly(t *testing.T) {
	path := writeTemp(t, minimalYAML)

	intents, err := LoadIntents(path)
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}

	if len(intents) != 2 {
		t.Fatalf("expected 2 intents, got %d", len(intents))
	}

	// --- first intent ---
	first := intents[0]
	if first.ID != "recent_purchases" {
		t.Errorf("intent[0].ID = %q, want %q", first.ID, "recent_purchases")
	}
	if first.Label != "Recent Purchases" {
		t.Errorf("intent[0].Label = %q, want %q", first.Label, "Recent Purchases")
	}
	if len(first.Actions) != 1 {
		t.Fatalf("intent[0]: expected 1 action, got %d", len(first.Actions))
	}

	action := first.Actions[0]
	if action.ActionID != "amazon.order_history" {
		t.Errorf("action.ActionID = %q, want %q", action.ActionID, "amazon.order_history")
	}
	if action.Site != "amazon" {
		t.Errorf("action.Site = %q, want %q", action.Site, "amazon")
	}
	if !action.IdentityRequired {
		t.Error("action.IdentityRequired should be true")
	}
	if action.OutputType != "transaction" {
		t.Errorf("action.OutputType = %q, want %q", action.OutputType, "transaction")
	}
	if v, ok := action.ParamsTemplate["days"]; !ok || v != 30 {
		t.Errorf("action.ParamsTemplate[days] = %v, want 30", v)
	}

	// --- second intent ---
	second := intents[1]
	if second.ID != "github_activity" {
		t.Errorf("intent[1].ID = %q, want %q", second.ID, "github_activity")
	}
	if second.Actions[0].IdentityRequired {
		t.Error("github_activity identity_required should be false")
	}
	if v := second.Actions[0].ParamsTemplate["username"]; v != "{user_input}" {
		t.Errorf("params_template[username] = %v, want '{user_input}'", v)
	}
}

func TestLoadIntents_FileNotFound(t *testing.T) {
	_, err := LoadIntents("/nonexistent/path/intents.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadIntents_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "intents: [this: is: broken: yaml::::")
	_, err := LoadIntents(path)
	if err == nil {
		t.Fatal("expected parse error for malformed YAML, got nil")
	}
}

func TestLoadIntents_UnknownField_Rejected(t *testing.T) {
	// KnownFields(true) should reject this.
	yaml := `
intents:
  - id: test
    label: "Test"
    description: "test"
    typo_field: "should fail"
    actions:
      - action_id: foo.bar
        site: foo
        params_template: {}
        identity_required: false
        output_type: generic
`
	path := writeTemp(t, yaml)
	_, err := LoadIntents(path)
	if err == nil {
		t.Fatal("expected error for unknown YAML field, got nil")
	}
}

func TestLoadIntents_InvalidOutputType(t *testing.T) {
	yaml := `
intents:
  - id: bad_type
    label: "Bad"
    description: "bad output type"
    actions:
      - action_id: foo.bar
        site: foo
        params_template: {}
        identity_required: false
        output_type: banana
`
	path := writeTemp(t, yaml)
	_, err := LoadIntents(path)
	if err == nil {
		t.Fatal("expected validation error for invalid output_type, got nil")
	}
}

func TestLoadIntents_DuplicateID_Rejected(t *testing.T) {
	yaml := `
intents:
  - id: dup
    label: "First"
    description: "first"
    actions:
      - action_id: foo.bar
        site: foo
        params_template: {}
        identity_required: false
        output_type: generic
  - id: dup
    label: "Second"
    description: "second"
    actions:
      - action_id: foo.baz
        site: foo
        params_template: {}
        identity_required: false
        output_type: generic
`
	path := writeTemp(t, yaml)
	_, err := LoadIntents(path)
	if err == nil {
		t.Fatal("expected error for duplicate intent ID, got nil")
	}
}

func TestLoadIntents_RealFile(t *testing.T) {
	// Smoke test against the actual intents.yaml in the repo.
	// Skips gracefully if running outside the project tree.
	path := filepath.Join("..", "intents", "intents.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try relative to test file location.
		path = "intents.yaml"
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("intents.yaml not found, skipping real-file smoke test")
	}

	intents, err := LoadIntents(path)
	if err != nil {
		t.Fatalf("LoadIntents(intents.yaml) failed: %v", err)
	}
	if len(intents) < 6 {
		t.Errorf("expected at least 6 intents in intents.yaml, got %d", len(intents))
	}
}

// ---- FindIntent tests ----

func TestFindIntent_ReturnsCorrectIntent(t *testing.T) {
	intents := []Intent{
		{ID: "alpha", Label: "Alpha", Actions: []Action{{ActionID: "a.b", OutputType: "generic"}}},
		{ID: "beta", Label: "Beta", Actions: []Action{{ActionID: "b.c", OutputType: "activity"}}},
	}

	got, err := FindIntent(intents, "beta")
	if err != nil {
		t.Fatalf("FindIntent failed: %v", err)
	}
	if got.ID != "beta" {
		t.Errorf("got ID %q, want %q", got.ID, "beta")
	}
	if got.Label != "Beta" {
		t.Errorf("got Label %q, want %q", got.Label, "Beta")
	}
}

func TestFindIntent_ReturnsPointerNotCopy(t *testing.T) {
	// Mutating the returned pointer should affect the slice element.
	intents := []Intent{
		{ID: "alpha", Label: "Original"},
	}
	got, err := FindIntent(intents, "alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got.Label = "Mutated"
	if intents[0].Label != "Mutated" {
		t.Error("FindIntent should return a pointer into the slice, not a copy")
	}
}

func TestFindIntent_UnknownID_ReturnsError(t *testing.T) {
	intents := []Intent{
		{ID: "known"},
	}
	_, err := FindIntent(intents, "unknown_id")
	if err == nil {
		t.Fatal("expected error for unknown intent ID, got nil")
	}
}

func TestFindIntent_EmptySlice_ReturnsError(t *testing.T) {
	_, err := FindIntent([]Intent{}, "anything")
	if err == nil {
		t.Fatal("expected error on empty intent slice, got nil")
	}
}
