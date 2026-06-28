package intents

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Action maps to a single Wire API call within an intent.
type Action struct {
	ActionID         string         `yaml:"action_id"`
	Site             string         `yaml:"site"`
	ParamsTemplate   map[string]any `yaml:"params_template"`
	IdentityRequired bool           `yaml:"identity_required"`
	OutputType       string         `yaml:"output_type"`
}

// Intent is one user-facing query type that fans out to one or more Wire actions.
type Intent struct {
	ID          string   `yaml:"id"`
	Label       string   `yaml:"label"`
	Description string   `yaml:"description"`
	Actions     []Action `yaml:"actions"`
}

type intentFile struct {
	Intents []Intent `yaml:"intents"`
}

// LoadIntents reads and parses the YAML routing table at path.
func LoadIntents(path string) ([]Intent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("intents: read file %q: %w", path, err)
	}

	var file intentFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("intents: parse yaml %q: %w", path, err)
	}

	if err := validate(file.Intents); err != nil {
		return nil, fmt.Errorf("intents: validation: %w", err)
	}

	return file.Intents, nil
}

// FindIntent returns the Intent with the given id, or a descriptive error if not found.
func FindIntent(intents []Intent, id string) (*Intent, error) {
	for i := range intents {
		if intents[i].ID == id {
			return &intents[i], nil
		}
	}
	return nil, fmt.Errorf("intents: unknown intent %q", id)
}

var validOutputTypes = map[string]bool{
	"transaction": true,
	"activity":    true,
	"contact":     true,
	"generic":     true,
}

func validate(intents []Intent) error {
	seen := make(map[string]bool, len(intents))
	for _, intent := range intents {
		if intent.ID == "" {
			return fmt.Errorf("intent missing id")
		}
		if seen[intent.ID] {
			return fmt.Errorf("duplicate intent id %q", intent.ID)
		}
		seen[intent.ID] = true

		if len(intent.Actions) == 0 {
			return fmt.Errorf("intent %q has no actions", intent.ID)
		}

		for _, action := range intent.Actions {
			if action.ActionID == "" {
				return fmt.Errorf("intent %q: action missing action_id", intent.ID)
			}
			if !validOutputTypes[action.OutputType] {
				return fmt.Errorf(
					"intent %q action %q: invalid output_type %q (must be one of: transaction, activity, contact, generic)",
					intent.ID, action.ActionID, action.OutputType,
				)
			}
		}
	}
	return nil
}
