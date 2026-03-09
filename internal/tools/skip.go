package tools

import (
	"io/fs"
	"strings"
)

// skipDirs lists directory names to skip during file walks.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".venv":        true,
	"__pycache__":  true,
	".idea":        true,
	".vscode":      true,
}

// shouldSkipDir returns true if the entry is a directory that should be skipped.
func shouldSkipDir(d fs.DirEntry) bool {
	if !d.IsDir() {
		return false
	}
	name := d.Name()
	if name == "." || name == ".." {
		return false
	}
	return skipDirs[name] || strings.HasPrefix(name, ".")
}
