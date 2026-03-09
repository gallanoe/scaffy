package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type GrepSearchTool struct{}

type grepSearchArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Include string `json:"include"`
}

func (t *GrepSearchTool) Name() string { return "grep_search" }

func (t *GrepSearchTool) Description() string {
	return "Search file contents using a regular expression. Returns matching lines as file:line:content."
}

func (t *GrepSearchTool) ParametersSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regular expression pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "Root directory to search from (defaults to current directory)"
			},
			"include": {
				"type": "string",
				"description": "File glob filter (e.g. \"*.go\" to only search Go files)"
			}
		},
		"required": ["pattern"]
	}`)
}

const maxGrepMatches = 500

func (t *GrepSearchTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var a grepSearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}

	root := a.Path
	if root == "" {
		root = "."
	}

	var results []string
	truncated := false

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if shouldSkipDir(d) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		// Apply include filter
		if a.Include != "" {
			matched, err := filepath.Match(a.Include, d.Name())
			if err != nil || !matched {
				return nil
			}
		}

		matches, err := grepFile(path, re)
		if err != nil {
			return nil // skip files we can't read
		}
		results = append(results, matches...)
		if len(results) >= maxGrepMatches {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("search error: %w", walkErr)
	}

	if len(results) == 0 {
		return "No matches found.", nil
	}

	output := strings.Join(results, "\n")
	if truncated {
		output += fmt.Sprintf("\n\n(truncated at %d matches)", maxGrepMatches)
	}
	return output, nil
}

func grepFile(path string, re *regexp.Regexp) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		// Skip binary content
		if !utf8.ValidString(line) {
			return nil, nil
		}
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:%s", path, lineNum, line))
		}
	}
	return matches, scanner.Err()
}
