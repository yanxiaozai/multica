package issuebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitLabClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type GitLabUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func NewGitLabClient(baseURL, token string) (*GitLabClient, error) {
	normalized, err := NormalizeGitLabBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &GitLabClient{
		baseURL: normalized,
		token:   strings.TrimSpace(token),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func NewGitLabClientWithHTTPClient(baseURL, token string, httpClient *http.Client) (*GitLabClient, error) {
	client, err := NewGitLabClient(baseURL, token)
	if err != nil {
		return nil, err
	}
	if httpClient != nil {
		client.httpClient = httpClient
	}
	return client, nil
}

func NormalizeGitLabBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("gitlab base URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid gitlab base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("gitlab base URL must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("gitlab base URL must include a host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("gitlab base URL must not include user info")
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	switch path {
	case "", "/":
		parsed.Path = ""
	case "/api/v4":
		parsed.Path = ""
	default:
		return "", fmt.Errorf("gitlab base URL must not include a path")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (c *GitLabClient) TestConnection(ctx context.Context) (GitLabUser, error) {
	if c == nil {
		return GitLabUser{}, fmt.Errorf("gitlab client is nil")
	}
	if strings.TrimSpace(c.token) == "" {
		return GitLabUser{}, fmt.Errorf("gitlab token is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v4/user", nil)
	if err != nil {
		return GitLabUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return GitLabUser{}, fmt.Errorf("gitlab connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return GitLabUser{}, fmt.Errorf("gitlab connection failed: status %d", resp.StatusCode)
	}
	var user GitLabUser
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user); err != nil {
		return GitLabUser{}, fmt.Errorf("decode gitlab user: %w", err)
	}
	return user, nil
}
