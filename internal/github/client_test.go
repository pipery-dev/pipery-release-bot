package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientUsesGitHubRESTAPIShape(t *testing.T) {
	var seen []seenRequest
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		seen = append(seen, seenRequest{
			Method:        r.Method,
			Path:          r.URL.EscapedPath(),
			RawQuery:      r.URL.RawQuery,
			Authorization: r.Header.Get("Authorization"),
			Accept:        r.Header.Get("Accept"),
			APIVersion:    r.Header.Get("X-GitHub-Api-Version"),
			ContentType:   r.Header.Get("Content-Type"),
			Body:          body,
		})

		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/pipery-dev/pipery/git/ref/heads/main":
			return jsonResponse(http.StatusOK, map[string]any{"ref": "refs/heads/main", "object": map[string]string{"sha": "abc123"}}), nil
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/repos/pipery-dev/pipery/git/refs":
			return jsonResponse(http.StatusCreated, map[string]string{}), nil
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/repos/pipery-dev/pipery/contents/docs/release%20notes.md":
			return jsonResponse(http.StatusOK, map[string]string{
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte("release notes")),
			}), nil
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/repos/pipery-dev/pipery/releases":
			return jsonResponse(http.StatusCreated, map[string]string{}), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.EscapedPath(), r.URL.RawQuery)
			return nil, nil
		}
	})

	client := NewClient(&http.Client{Transport: transport}, staticAuth{token: "test-token"})
	client.baseURL = "https://api.github.test"

	ref, err := client.GetRef(context.Background(), "default", 7, "pipery-dev", "pipery", "heads/main")
	if err != nil {
		t.Fatalf("GetRef returned error: %v", err)
	}
	if ref.Ref != "refs/heads/main" || ref.SHA != "abc123" {
		t.Fatalf("ref = %+v, want refs/heads/main abc123", ref)
	}
	if err := client.CreateRef(context.Background(), "default", 7, "pipery-dev", "pipery", "refs/heads/release/v1.2.3", "abc123"); err != nil {
		t.Fatalf("CreateRef returned error: %v", err)
	}
	contents, err := client.GetContents(context.Background(), "default", 7, "pipery-dev", "pipery", "docs/release notes.md", "release/v1.2.3")
	if err != nil {
		t.Fatalf("GetContents returned error: %v", err)
	}
	if contents != "release notes" {
		t.Fatalf("contents = %q, want release notes", contents)
	}
	if err := client.CreateRelease(context.Background(), "default", 7, "pipery-dev", "pipery", "v1.2.3", "v1.2.3", "release notes"); err != nil {
		t.Fatalf("CreateRelease returned error: %v", err)
	}

	if len(seen) != 4 {
		t.Fatalf("saw %d requests, want 4", len(seen))
	}
	for _, req := range seen {
		if req.Authorization != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", req.Authorization)
		}
		if req.Accept != "application/vnd.github+json" {
			t.Fatalf("Accept = %q, want GitHub JSON media type", req.Accept)
		}
		if req.APIVersion != "2022-11-28" {
			t.Fatalf("X-GitHub-Api-Version = %q, want 2022-11-28", req.APIVersion)
		}
	}
	if seen[1].ContentType != "application/json" || seen[1].Body["ref"] != "refs/heads/release/v1.2.3" || seen[1].Body["sha"] != "abc123" {
		t.Fatalf("CreateRef request = %+v", seen[1])
	}
	if seen[2].RawQuery != "ref=release%2Fv1.2.3" {
		t.Fatalf("contents query = %q, want escaped release ref", seen[2].RawQuery)
	}
	if seen[3].ContentType != "application/json" || seen[3].Body["tag_name"] != "v1.2.3" || seen[3].Body["body"] != "release notes" {
		t.Fatalf("CreateRelease request = %+v", seen[3])
	}
}

func TestClientReturnsGitHubErrorBody(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return textResponse(http.StatusUnprocessableEntity, "bad ref\n"), nil
	})}, staticAuth{token: "test-token"})
	client.baseURL = "https://api.github.test"

	_, err := client.GetRef(context.Background(), "default", 7, "pipery-dev", "pipery", "heads/main")
	if err == nil {
		t.Fatal("expected GitHub status error")
	}
}

func TestGetContentsRejectsUnsupportedEncoding(t *testing.T) {
	client := NewClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]string{"encoding": "none", "content": "plain"}), nil
	})}, staticAuth{token: "test-token"})
	client.baseURL = "https://api.github.test"

	if _, err := client.GetContents(context.Background(), "default", 7, "pipery-dev", "pipery", "CHANGELOG.md", "main"); err == nil {
		t.Fatal("expected unsupported encoding error")
	}
}

type staticAuth struct {
	token string
}

func (a staticAuth) Token(context.Context, string, int64) (string, error) {
	return a.token, nil
}

type seenRequest struct {
	Method        string
	Path          string
	RawQuery      string
	Authorization string
	Accept        string
	APIVersion    string
	ContentType   string
	Body          map[string]any
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, value any) *http.Response {
	var b strings.Builder
	_ = json.NewEncoder(&b).Encode(value)
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(b.String())),
	}
}

func textResponse(status int, value string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(value)),
	}
}
