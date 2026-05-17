package release

import (
	"context"
	"fmt"
	"strings"

	"github.com/pipery-dev/pipery-release-bot/internal/config"
)

type GitHub interface {
	GetRef(ctx context.Context, installationKey string, installationID int64, owner, repo, ref string) (GitRef, error)
	CreateRef(ctx context.Context, installationKey string, installationID int64, owner, repo, ref, sha string) error
	CreateRelease(ctx context.Context, installationKey string, installationID int64, owner, repo, tagName, name, body string) error
	GetContents(ctx context.Context, installationKey string, installationID int64, owner, repo, path, ref string) (string, error)
}

type GitRef struct {
	Ref string
	SHA string
}

type ExecuteRequest struct {
	InstallationKey  string `json:"installation_key"`
	InstallationID   int64  `json:"installation_id"`
	Owner            string `json:"owner"`
	Repo             string `json:"repo"`
	BaseRef          string `json:"base_ref"`
	Version          string `json:"version"`
	ReleaseNotesPath string `json:"release_notes_path"`
}

type ExecuteResult struct {
	Owner    string         `json:"owner"`
	Repo     string         `json:"repo"`
	BaseRef  string         `json:"base_ref"`
	BaseSHA  string         `json:"base_sha"`
	Branches []BranchResult `json:"branches"`
}

type BranchResult struct {
	Branch         string `json:"branch"`
	Tag            string `json:"tag,omitempty"`
	ReleaseCreated bool   `json:"release_created"`
}

type Service struct {
	target   config.Target
	patterns []config.BranchPattern
	github   GitHub
}

func NewService(target config.Target, patterns []config.BranchPattern, github GitHub) *Service {
	return &Service{target: target, patterns: patterns, github: github}
}

func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	req = s.withDefaults(req)
	if req.InstallationKey == "" {
		req.InstallationKey = "default"
	}
	if req.Owner == "" || req.Repo == "" || req.BaseRef == "" || req.Version == "" {
		return ExecuteResult{}, fmt.Errorf("owner, repo, base_ref, and version are required")
	}
	if err := validateBranchName(req.BaseRef); err != nil {
		return ExecuteResult{}, fmt.Errorf("invalid base_ref: %w", err)
	}

	base, err := s.github.GetRef(ctx, req.InstallationKey, req.InstallationID, req.Owner, req.Repo, "heads/"+req.BaseRef)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("get base ref: %w", err)
	}
	if base.SHA == "" {
		return ExecuteResult{}, fmt.Errorf("base ref returned an empty SHA")
	}

	result := ExecuteResult{
		Owner:   req.Owner,
		Repo:    req.Repo,
		BaseRef: req.BaseRef,
		BaseSHA: base.SHA,
	}

	var releaseNotes string
	for _, pattern := range s.patterns {
		branch, err := renderPattern(pattern.Pattern, req.Version)
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("render branch pattern %q: %w", pattern.Pattern, err)
		}

		if err := s.github.CreateRef(ctx, req.InstallationKey, req.InstallationID, req.Owner, req.Repo, "refs/heads/"+branch, base.SHA); err != nil {
			return ExecuteResult{}, fmt.Errorf("create branch %q: %w", branch, err)
		}

		item := BranchResult{Branch: branch}
		if pattern.CreateTag {
			tagName := renderTagName(pattern.TagName, req.Version)
			if err := validateTagName(tagName); err != nil {
				return ExecuteResult{}, fmt.Errorf("invalid tag for branch %q: %w", branch, err)
			}
			if err := s.github.CreateRef(ctx, req.InstallationKey, req.InstallationID, req.Owner, req.Repo, "refs/tags/"+tagName, base.SHA); err != nil {
				return ExecuteResult{}, fmt.Errorf("create tag %q: %w", tagName, err)
			}
			item.Tag = tagName

			if pattern.CreateRelease {
				if releaseNotes == "" && req.ReleaseNotesPath != "" {
					releaseNotes, err = s.github.GetContents(ctx, req.InstallationKey, req.InstallationID, req.Owner, req.Repo, req.ReleaseNotesPath, req.BaseRef)
					if err != nil {
						return ExecuteResult{}, fmt.Errorf("read release notes: %w", err)
					}
				}
				if err := s.github.CreateRelease(ctx, req.InstallationKey, req.InstallationID, req.Owner, req.Repo, tagName, tagName, releaseNotes); err != nil {
					return ExecuteResult{}, fmt.Errorf("create release %q: %w", tagName, err)
				}
				item.ReleaseCreated = true
			}
		}
		result.Branches = append(result.Branches, item)
	}

	return result, nil
}

func (s *Service) withDefaults(req ExecuteRequest) ExecuteRequest {
	if req.Owner == "" {
		req.Owner = s.target.Owner
	}
	if req.Repo == "" {
		req.Repo = s.target.Repo
	}
	if req.BaseRef == "" {
		req.BaseRef = s.target.BaseRef
	}
	if req.Version == "" {
		req.Version = s.target.Version
	}
	if req.ReleaseNotesPath == "" {
		req.ReleaseNotesPath = s.target.ReleaseNotesPath
	}
	return req
}

func renderTagName(pattern, version string) string {
	if pattern == "" {
		return version
	}
	return strings.ReplaceAll(pattern, "{version}", version)
}

func validateTagName(tag string) error {
	return validateBranchName(tag)
}
