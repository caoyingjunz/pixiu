package types

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	MaxRepositoryFiles       = 20
	MaxRepositoryFilenameLen = 255
	MaxRepositoryFileSize    = 1 << 20
)

// CleanRepositoryFiles validates repository override files and returns a copy
// with surrounding whitespace removed from filenames. Content is unchanged.
func CleanRepositoryFiles(files map[string]string) (map[string]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if len(files) > MaxRepositoryFiles {
		return nil, fmt.Errorf("repository files cannot exceed %d", MaxRepositoryFiles)
	}
	cleaned := make(map[string]string, len(files))
	for rawName, content := range files {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return nil, fmt.Errorf("repository filename cannot be empty")
		}
		if len(name) > MaxRepositoryFilenameLen {
			return nil, fmt.Errorf("repository filename %q exceeds %d bytes", name, MaxRepositoryFilenameLen)
		}
		if name == "." || name == ".." || strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
			return nil, fmt.Errorf("invalid repository filename %q", name)
		}
		if content == "" {
			return nil, fmt.Errorf("repository file %q content cannot be empty", name)
		}
		if len(content) > MaxRepositoryFileSize {
			return nil, fmt.Errorf("repository file %q exceeds %d bytes", name, MaxRepositoryFileSize)
		}
		if _, exists := cleaned[name]; exists {
			return nil, fmt.Errorf("duplicate repository filename %q", name)
		}
		cleaned[name] = content
	}
	return cleaned, nil
}
