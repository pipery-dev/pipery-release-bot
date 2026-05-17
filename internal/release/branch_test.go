package release

import "testing"

func TestRenderPatternValidatesBranchName(t *testing.T) {
	branch, err := renderPattern("release/{version}", "v1.2.3")
	if err != nil {
		t.Fatalf("renderPattern returned error: %v", err)
	}
	if branch != "release/v1.2.3" {
		t.Fatalf("branch = %q, want release/v1.2.3", branch)
	}
}

func TestRenderPatternRejectsUnsafeBranchName(t *testing.T) {
	_, err := renderPattern("release/{version}", "bad..ref")
	if err == nil {
		t.Fatal("expected unsafe branch name to be rejected")
	}
}
