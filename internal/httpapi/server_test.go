package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pipery-dev/pipery-release-bot/internal/config"
	"github.com/pipery-dev/pipery-release-bot/internal/release"
)

func TestRoutesHealthz(t *testing.T) {
	server := NewServer(release.NewService(config.Target{}, nil, nil))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestExecuteRejectsInvalidJSON(t *testing.T) {
	server := NewServer(release.NewService(config.Target{}, nil, nil))
	req := httptest.NewRequest(http.MethodPost, "/v1/release-plans/execute", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorResponse(t, rec.Body.String(), "invalid JSON")
}

func TestBearerAuthProtectsExecuteButNotHealth(t *testing.T) {
	server := NewServer(release.NewService(config.Target{}, nil, nil), "secret-token")

	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/release-plans/execute", strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("execute status = %d, want 401", rec.Code)
	}
}

func TestExecuteReturnsServiceValidationErrors(t *testing.T) {
	server := NewServer(release.NewService(config.Target{}, []config.BranchPattern{{Pattern: "release/{version}"}}, &handlerGitHub{}))
	req := httptest.NewRequest(http.MethodPost, "/v1/release-plans/execute", strings.NewReader(`{"owner":"pipery-dev"}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	assertErrorResponse(t, rec.Body.String(), "owner, repo, base_ref, and version are required")
}

func TestExecuteSuccess(t *testing.T) {
	server := NewServer(release.NewService(
		config.Target{},
		[]config.BranchPattern{{Pattern: "release/{version}", CreateTag: true, TagName: "{version}"}},
		&handlerGitHub{base: release.GitRef{Ref: "refs/heads/main", SHA: "abc123"}},
	))
	req := httptest.NewRequest(http.MethodPost, "/v1/release-plans/execute", strings.NewReader(`{
		"owner":"pipery-dev",
		"repo":"pipery",
		"base_ref":"main",
		"version":"v1.2.3"
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var result release.ExecuteResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.BaseSHA != "abc123" || len(result.Branches) != 1 || result.Branches[0].Branch != "release/v1.2.3" || result.Branches[0].Tag != "v1.2.3" {
		t.Fatalf("unexpected execute result: %+v", result)
	}
}

func assertErrorResponse(t *testing.T, body, want string) {
	t.Helper()
	var parsed map[string]string
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("decode error response: %v; body: %s", err, body)
	}
	if !strings.Contains(parsed["error"], want) {
		t.Fatalf("error = %q, want to contain %q", parsed["error"], want)
	}
}

type handlerGitHub struct {
	base release.GitRef
}

func (g *handlerGitHub) GetRef(context.Context, string, int64, string, string, string) (release.GitRef, error) {
	return g.base, nil
}

func (g *handlerGitHub) CreateRef(context.Context, string, int64, string, string, string, string) error {
	return nil
}

func (g *handlerGitHub) CreateRelease(context.Context, string, int64, string, string, string, string, string) error {
	return nil
}

func (g *handlerGitHub) GetContents(context.Context, string, int64, string, string, string, string) (string, error) {
	return "", nil
}
