package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pipery-dev/pipery-release-bot/internal/release"
)

type Client struct {
	httpClient *http.Client
	auth       Authenticator
	baseURL    string
}

func NewClient(httpClient *http.Client, auth Authenticator) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, auth: auth, baseURL: "https://api.github.com"}
}

func (c *Client) GetRef(ctx context.Context, installationKey string, installationID int64, owner, repo, ref string) (release.GitRef, error) {
	var parsed struct {
		Ref    string `json:"ref"`
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	path := fmt.Sprintf("/repos/%s/%s/git/ref/%s", url.PathEscape(owner), url.PathEscape(repo), escapeRef(ref))
	if err := c.do(ctx, installationKey, installationID, http.MethodGet, path, nil, &parsed); err != nil {
		return release.GitRef{}, err
	}
	return release.GitRef{Ref: parsed.Ref, SHA: parsed.Object.SHA}, nil
}

func (c *Client) CreateRef(ctx context.Context, installationKey string, installationID int64, owner, repo, ref, sha string) error {
	body := map[string]string{"ref": ref, "sha": sha}
	path := fmt.Sprintf("/repos/%s/%s/git/refs", url.PathEscape(owner), url.PathEscape(repo))
	return c.do(ctx, installationKey, installationID, http.MethodPost, path, body, nil)
}

func (c *Client) GetContents(ctx context.Context, installationKey string, installationID int64, owner, repo, pathValue, ref string) (string, error) {
	var parsed struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	path := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", url.PathEscape(owner), url.PathEscape(repo), escapePath(pathValue), url.QueryEscape(ref))
	if err := c.do(ctx, installationKey, installationID, http.MethodGet, path, nil, &parsed); err != nil {
		return "", err
	}
	if parsed.Encoding != "base64" {
		return "", fmt.Errorf("unsupported contents encoding %q", parsed.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(parsed.Content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) CreateRelease(ctx context.Context, installationKey string, installationID int64, owner, repo, tagName, name, bodyText string) error {
	body := map[string]any{
		"tag_name": tagName,
		"name":     name,
		"body":     bodyText,
	}
	path := fmt.Sprintf("/repos/%s/%s/releases", url.PathEscape(owner), url.PathEscape(repo))
	return c.do(ctx, installationKey, installationID, http.MethodPost, path, body, nil)
}

func (c *Client) do(ctx context.Context, installationKey string, installationID int64, method, path string, in, out any) error {
	token, err := c.auth.Token(ctx, installationKey, installationID)
	if err != nil {
		return err
	}

	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github %s %s returned %s: %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func escapeRef(ref string) string {
	return strings.ReplaceAll(url.PathEscape(ref), "%2F", "/")
}

func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
