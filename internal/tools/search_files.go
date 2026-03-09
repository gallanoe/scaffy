package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type SearchFilesTool struct{}

type searchFilesArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (t *SearchFilesTool) Name() string { return "search_files" }

func (t *SearchFilesTool) Description() string {
	return "Search for files matching a glob pattern. Returns matching file paths."
}

func (t *SearchFilesTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match files (e.g. \"*.go\", \"**/*.yaml\")"
			},
			"path": {
				"type": "string",
				"description": "Root directory to search from (defaults to current directory)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *SearchFilesTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var a searchFilesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	root := a.Path
	if root == "" {
		root = "."
	}

	// Validate the pattern
	if _, err := filepath.Match(a.Pattern, "test"); err != nil {
		return "", fmt.Errorf("invalid glob pattern: %w", err)
	}

	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't access
		}
		if shouldSkipDir(d) {
			return filepath.SkipDir
		}
		matched, err := filepath.Match(a.Pattern, d.Name())
		if err != nil {
			return nil
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search error: %w", err)
	}

	if len(matches) == 0 {
		return "No files found matching pattern.", nil
	}
	return strings.Join(matches, "\n"), nil
}
