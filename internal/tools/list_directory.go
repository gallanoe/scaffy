package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type ListDirectoryTool struct{}

type listDirectoryArgs struct {
	Path string `json:"path"`
}

func (t *ListDirectoryTool) Name() string { return "list_directory" }

func (t *ListDirectoryTool) Description() string {
	return "List the contents of a directory, showing files and subdirectories."
}

func (t *ListDirectoryTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Path to the directory to list"
			}
		},
		"required": ["path"]
	}`)
}

func (t *ListDirectoryTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var a listDirectoryArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	entries, err := os.ReadDir(a.Path)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}

	var b strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Fprintf(&b, "%s/\n", entry.Name())
		} else {
			fmt.Fprintf(&b, "%s\n", entry.Name())
		}
	}
	return b.String(), nil
}
