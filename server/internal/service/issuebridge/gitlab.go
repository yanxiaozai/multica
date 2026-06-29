package issuebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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

// GitLabIssue is the subset of a GitLab issue the bridge imports. Field
// names match the GitLab REST API JSON (`iid`, `title`, `description`,
// `state`, `web_url`, `updated_at`).
type GitLabIssue struct {
	IID          int64          `json:"iid"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	State        string         `json:"state"` // "opened" | "closed"
	WebURL       string         `json:"web_url"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Assignees    []GitLabUser   `json:"assignees"`
}

// ListIssuesOpts filters and bounds a ListProjectIssues call.
type ListIssuesOpts struct {
	// AssignedToMe adds scope=assigned_to_me, restricting to issues
	// assigned to the token owner (the "本人" of the connection).
	AssignedToMe bool
	// State filters by GitLab state: "opened", "closed", or "all"
	// (empty = all). Phase A imports use "all" so closed issues land as
	// `done` via state_mapping.
	State string
	// UpdatedAfter (Phase B) requests only issues updated since this
	// timestamp for incremental polling. Zero value = no filter.
	UpdatedAfter time.Time
	// PerPage is the page size (GitLab max 100). Defaults to 100.
	PerPage int
	// MaxPages caps pagination so a huge project can't stall an import
	// forever. Defaults to 20 (up to 2000 issues).
	MaxPages int
}

// defaultListHTTPClient is a separate client from the 10s TestConnection
// one — paginating a large project can legitimately take longer, so list
// calls get a 30s budget.
const listIssueTimeout = 30 * time.Second
const defaultListPerPage = 100
const defaultListMaxPages = 20

// ListProjectIssues fetches issues from a GitLab project, following
// pagination up to opts.MaxPages. projectRef is the URL-encoded project
// path or id (e.g. "group/subgroup/proj" or "42"). Returns the aggregated
// issues across all visited pages.
func (c *GitLabClient) ListProjectIssues(ctx context.Context, projectRef string, opts ListIssuesOpts) ([]GitLabIssue, error) {
	if c == nil {
		return nil, fmt.Errorf("gitlab client is nil")
	}
	if strings.TrimSpace(c.token) == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}
	ref := NormalizeProjectRef(projectRef)
	if ref == "" {
		return nil, fmt.Errorf("gitlab project ref is required")
	}
	if opts.PerPage <= 0 {
		opts.PerPage = defaultListPerPage
	}
	if opts.PerPage > 100 {
		opts.PerPage = 100
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = defaultListMaxPages
	}

	endpoint := c.baseURL + "/api/v4/projects/" + url.PathEscape(ref) + "/issues"
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: listIssueTimeout}
	}

	var all []GitLabIssue
	page := 1
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("per_page", strconv.Itoa(opts.PerPage))
		q.Set("page", strconv.Itoa(page))
		// Order by created_at ascending so the import matches GitLab's
		// chronological issue numbering — deterministic across re-runs.
		q.Set("order_by", "created_at")
		q.Set("sort", "asc")
		if opts.AssignedToMe {
			q.Set("scope", "assigned_to_me")
		}
		if s := strings.TrimSpace(opts.State); s != "" && s != "all" {
			q.Set("state", s)
		}
		if !opts.UpdatedAfter.IsZero() {
			q.Set("updated_after", opts.UpdatedAfter.UTC().Format(time.RFC3339))
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("gitlab list issues failed: %w", err)
		}
		// Read the body once so an error response can surface GitLab's reason
		// (e.g. {"message":"404 Project Not Found"}) instead of a bare status.
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("gitlab list issues: read response page %d: %w", page, readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, httpErrorForStatus(resp.StatusCode, body)
		}

		var pageIssues []GitLabIssue
		if err := json.Unmarshal(body, &pageIssues); err != nil {
			return nil, fmt.Errorf("decode gitlab issues page %d: %w", page, err)
		}

		all = append(all, pageIssues...)
		// X-Next-Page is authoritative when present. Fall back to "short
		// page = done" only when a proxy stripped the header — otherwise a
		// sparse intermediate page would wrongly terminate pagination.
		nextHeader := resp.Header.Get("X-Next-Page")
		nextPage, _ := strconv.Atoi(nextHeader)
		if nextHeader == "" {
			if len(pageIssues) < opts.PerPage {
				break
			}
			nextPage = page + 1
		}
		if nextPage <= 0 || page >= opts.MaxPages {
			break
		}
		page = nextPage
	}
	return all, nil
}

// NormalizeProjectRef cleans a user-entered GitLab project reference into the
// form the API expects: "group/subgroup/project" or a numeric id. It accepts
// pasted full URLs (https://gitlab.example.com/group/proj or …/group/proj/-/issues),
// trims stray slashes, and strips a trailing .git — all common entry mistakes
// that would otherwise 404. Returns "" if the input is empty after cleaning.
func NormalizeProjectRef(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// If the user pasted a full URL, keep only the path. The configured
	// integration base URL supplies the host; the pasted host is ignored.
	if u, err := url.Parse(s); err == nil && u.Scheme != "" && u.Host != "" {
		s = strings.Trim(u.EscapedPath(), "/")
		// Drop everything from "/-/" onwards — GitLab UI sub-pages
		// (issues, merge requests, …) all share that sentinel.
		if i := strings.Index(s, "/-/"); i >= 0 {
			s = s[:i]
		}
	} else {
		s = strings.Trim(s, "/")
	}
	s = strings.TrimSuffix(s, ".git")
	return strings.Trim(s, "/")
}

// httpErrorForStatus turns a non-2xx GitLab response into an error that
// carries GitLab's own message body (capped) plus, for the common 404/403, an
// actionable hint. The body is where GitLab states the real reason
// ("404 Project Not Found", "401 Unauthorized", rate-limit JSON, …).
func httpErrorForStatus(status int, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 300 {
		snippet = snippet[:300]
	}
	switch status {
	case http.StatusNotFound:
		// GitLab returns 404 (not 403) for a project the token can't see, so
		// the two root causes — wrong path vs. no access — look identical.
		return fmt.Errorf("gitlab project not found (status 404): check the project path is correct (e.g. group/project) and that the token can access it%s", bodySuffix(snippet))
	case http.StatusForbidden, http.StatusUnauthorized:
		return fmt.Errorf("gitlab access denied (status %d): the token lacks access or is invalid%s", status, bodySuffix(snippet))
	default:
		return fmt.Errorf("gitlab list issues failed: status %d%s", status, bodySuffix(snippet))
	}
}

// bodySuffix formats the trimmed GitLab body for inclusion in an error,
// omitting it entirely when empty so a clean error reads naturally.
func bodySuffix(snippet string) string {
	if snippet == "" {
		return ""
	}
	return ": " + snippet
}
