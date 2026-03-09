package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type EditFileTool struct{}

type editFileArgs struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditFileTool) Name() string { return "edit_file" }

func (t *EditFileTool) Description() string {
	return "Edit a file by replacing an exact string match. The old_string must appear exactly once in the file."
}

func (t *EditFileTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the file to edit"
			},
			"old_string": {
				"type": "string",
				"description": "The exact string to find and replace (must appear exactly once)"
			},
			"new_string": {
				"type": "string",
				"description": "The string to replace old_string with"
			}
		},
		"required": ["path", "old_string", "new_string"]
	}`)
}

func (t *EditFileTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var a editFileArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	count := strings.Count(content, a.OldString)
	switch count {
	case 0:
		return "", fmt.Errorf("old_string not found in %s", a.Path)
	case 1:
		// exactly one match, proceed
	default:
		return "", fmt.Errorf("old_string appears %d times in %s (must appear exactly once)", count, a.Path)
	}

	newContent := strings.Replace(content, a.OldString, a.NewString, 1)
	if err := os.WriteFile(a.Path, []byte(newContent), 0o600); err != nil { //#nosec G703 -- path comes from LLM tool call, intentional
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", a.Path), nil
}
