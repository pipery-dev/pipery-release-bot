package release

import (
	"fmt"
	"strings"
	"unicode"
)

func renderPattern(pattern, version string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("branch pattern is required")
	}
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	branch := strings.ReplaceAll(pattern, "{version}", version)
	if err := validateBranchName(branch); err != nil {
		return "", err
	}
	return branch, nil
}

func validateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}
	if len(branch) > 255 {
		return fmt.Errorf("branch name is too long")
	}
	if strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "//") {
		return fmt.Errorf("branch name has invalid slash placement")
	}
	if strings.HasPrefix(branch, ".") || strings.HasSuffix(branch, ".") {
		return fmt.Errorf("branch name has invalid dot placement")
	}
	if strings.HasSuffix(branch, ".lock") || strings.Contains(branch, ".lock/") {
		return fmt.Errorf("branch name cannot contain .lock path components")
	}
	if strings.Contains(branch, "..") || strings.Contains(branch, "@{") {
		return fmt.Errorf("branch name contains an unsafe sequence")
	}

	for _, r := range branch {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return fmt.Errorf("branch name contains whitespace or control characters")
		}
		switch r {
		case '~', '^', ':', '?', '*', '[', '\\':
			return fmt.Errorf("branch name contains invalid character %q", r)
		}
	}
	return nil
}
