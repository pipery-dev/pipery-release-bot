package release

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pipery-dev/pipery-release-bot/internal/config"
)

func TestExecuteExpandsReleasePlanAndReusesReleaseNotes(t *testing.T) {
	github := &fakeGitHub{
		base:     GitRef{Ref: "refs/heads/main", SHA: "abc123"},
		contents: "# v1.2.3\n\nShip it.\n",
	}
	svc := NewService(
		config.Target{
			Owner:            "pipery-dev",
			Repo:             "pipery",
			BaseRef:          "main",
			Version:          "v1.2.3",
			ReleaseNotesPath: "CHANGELOG.md",
		},
		[]config.BranchPattern{
			{Pattern: "release/{version}", CreateTag: true, TagName: "{version}", CreateRelease: true},
			{Pattern: "maintenance/{version}", CreateTag: true, TagName: "stable-{version}", CreateRelease: true},
			{Pattern: "staging/{version}"},
		},
		github,
	)

	result, err := svc.Execute(context.Background(), ExecuteRequest{InstallationID: 42})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if result.Owner != "pipery-dev" || result.Repo != "pipery" || result.BaseRef != "main" || result.BaseSHA != "abc123" {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	wantBranches := []BranchResult{
		{Branch: "release/v1.2.3", Tag: "v1.2.3", ReleaseCreated: true},
		{Branch: "maintenance/v1.2.3", Tag: "stable-v1.2.3", ReleaseCreated: true},
		{Branch: "staging/v1.2.3"},
	}
	if !reflect.DeepEqual(result.Branches, wantBranches) {
		t.Fatalf("branches = %+v, want %+v", result.Branches, wantBranches)
	}

	wantCalls := []string{
		"get-ref default 42 pipery-dev/pipery heads/main",
		"create-ref default 42 pipery-dev/pipery refs/heads/release/v1.2.3 abc123",
		"create-ref default 42 pipery-dev/pipery refs/tags/v1.2.3 abc123",
		"get-contents default 42 pipery-dev/pipery CHANGELOG.md main",
		"create-release default 42 pipery-dev/pipery v1.2.3 # v1.2.3\n\nShip it.\n",
		"create-ref default 42 pipery-dev/pipery refs/heads/maintenance/v1.2.3 abc123",
		"create-ref default 42 pipery-dev/pipery refs/tags/stable-v1.2.3 abc123",
		"create-release default 42 pipery-dev/pipery stable-v1.2.3 # v1.2.3\n\nShip it.\n",
		"create-ref default 42 pipery-dev/pipery refs/heads/staging/v1.2.3 abc123",
	}
	if !reflect.DeepEqual(github.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", github.calls, wantCalls)
	}
	if github.contentsReads != 1 {
		t.Fatalf("release notes reads = %d, want 1", github.contentsReads)
	}
}

func TestExecuteRejectsInvalidBaseRefBeforeCallingGitHub(t *testing.T) {
	github := &fakeGitHub{}
	svc := NewService(config.Target{}, []config.BranchPattern{{Pattern: "release/{version}"}}, github)

	_, err := svc.Execute(context.Background(), ExecuteRequest{
		Owner:   "pipery-dev",
		Repo:    "pipery",
		BaseRef: "main..bad",
		Version: "v1.2.3",
	})
	if err == nil {
		t.Fatal("expected invalid base_ref error")
	}
	if len(github.calls) != 0 {
		t.Fatalf("github was called before validation failed: %#v", github.calls)
	}
}

func TestExecutePropagatesReleaseNotesErrors(t *testing.T) {
	github := &fakeGitHub{
		base:        GitRef{Ref: "refs/heads/main", SHA: "abc123"},
		contentsErr: errors.New("not found"),
	}
	svc := NewService(
		config.Target{},
		[]config.BranchPattern{{Pattern: "release/{version}", CreateTag: true, CreateRelease: true}},
		github,
	)

	_, err := svc.Execute(context.Background(), ExecuteRequest{
		Owner:            "pipery-dev",
		Repo:             "pipery",
		BaseRef:          "main",
		Version:          "v1.2.3",
		ReleaseNotesPath: "CHANGELOG.md",
	})
	if err == nil {
		t.Fatal("expected release notes error")
	}
}

func TestRenderPatternValidationCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		version string
		want    string
		wantErr bool
	}{
		{name: "nested safe branch", pattern: "release/{version}/candidate", version: "v1.2.3", want: "release/v1.2.3/candidate"},
		{name: "missing pattern", version: "v1.2.3", wantErr: true},
		{name: "missing version", pattern: "release/{version}", wantErr: true},
		{name: "unsafe version", pattern: "release/{version}", version: "bad@{ref", wantErr: true},
		{name: "invalid slash placement", pattern: "/release/{version}", version: "v1.2.3", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderPattern(tt.pattern, tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("renderPattern returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("branch = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateBranchNameRejectsGitUnsafeForms(t *testing.T) {
	invalid := []string{
		"",
		".hidden",
		"release.",
		"release//v1",
		"release/v1.lock",
		"release/v 1",
		"release/v1~",
		"release/v1^",
		"release/v1:",
		"release/v1?",
		"release/v1*",
		"release/[v1]",
		`release\v1`,
	}

	for _, branch := range invalid {
		t.Run(branch, func(t *testing.T) {
			if err := validateBranchName(branch); err == nil {
				t.Fatal("expected branch to be rejected")
			}
		})
	}
}

type fakeGitHub struct {
	base          GitRef
	baseErr       error
	contents      string
	contentsErr   error
	calls         []string
	contentsReads int
}

func (f *fakeGitHub) GetRef(_ context.Context, installationKey string, installationID int64, owner, repo, ref string) (GitRef, error) {
	f.calls = append(f.calls, "get-ref "+installationKey+" "+itoa(installationID)+" "+owner+"/"+repo+" "+ref)
	if f.baseErr != nil {
		return GitRef{}, f.baseErr
	}
	return f.base, nil
}

func (f *fakeGitHub) CreateRef(_ context.Context, installationKey string, installationID int64, owner, repo, ref, sha string) error {
	f.calls = append(f.calls, "create-ref "+installationKey+" "+itoa(installationID)+" "+owner+"/"+repo+" "+ref+" "+sha)
	return nil
}

func (f *fakeGitHub) CreateRelease(_ context.Context, installationKey string, installationID int64, owner, repo, tagName, _, body string) error {
	f.calls = append(f.calls, "create-release "+installationKey+" "+itoa(installationID)+" "+owner+"/"+repo+" "+tagName+" "+body)
	return nil
}

func (f *fakeGitHub) GetContents(_ context.Context, installationKey string, installationID int64, owner, repo, path, ref string) (string, error) {
	f.calls = append(f.calls, "get-contents "+installationKey+" "+itoa(installationID)+" "+owner+"/"+repo+" "+path+" "+ref)
	f.contentsReads++
	if f.contentsErr != nil {
		return "", f.contentsErr
	}
	return f.contents, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
