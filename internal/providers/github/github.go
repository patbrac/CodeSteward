// Package github detects the GitHub Actions environment and posts/updates PR
// comments. It uses only the standard library net/http client and honors the
// caller-supplied context and a 30s default timeout.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codesteward-ai/codesteward/internal/report"
)

// Env is the detected GitHub Actions environment.
type Env struct {
	IsActions   bool
	Repo        string
	PRNumber    int
	BaseRef     string
	HeadRef     string
	EventPath   string
	APIURL      string
	Token       string
	Description string
}

// defaultAPIURL is used when GITHUB_API_URL is not set.
const defaultAPIURL = "https://api.github.com"

// ghEvent is the subset of the GitHub Actions event payload CodeSteward reads.
type ghEvent struct {
	PullRequest *struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
		Base   struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
}

// DetectEnv inspects environment variables (via getenv) to detect a GitHub
// Actions run. It reads GITHUB_ACTIONS, GITHUB_REPOSITORY, GITHUB_API_URL
// (defaulting to https://api.github.com), GITHUB_TOKEN, GITHUB_BASE_REF,
// GITHUB_HEAD_REF, and GITHUB_EVENT_PATH. When an event path is set, the JSON
// payload is parsed for the pull_request number, body, and base/head refs; the
// event's base/head refs fill in only when the corresponding env vars are
// empty. A missing or malformed event file is a fatal, actionable error.
func DetectEnv(getenv func(string) string) (*Env, error) {
	apiURL := getenv("GITHUB_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	env := &Env{
		IsActions: getenv("GITHUB_ACTIONS") == "true",
		Repo:      getenv("GITHUB_REPOSITORY"),
		APIURL:    apiURL,
		Token:     getenv("GITHUB_TOKEN"),
		BaseRef:   getenv("GITHUB_BASE_REF"),
		HeadRef:   getenv("GITHUB_HEAD_REF"),
		EventPath: getenv("GITHUB_EVENT_PATH"),
	}
	if env.EventPath != "" {
		data, err := os.ReadFile(env.EventPath)
		if err != nil {
			return nil, fmt.Errorf("github: reading event file %q: %w", env.EventPath, err)
		}
		var ev ghEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, fmt.Errorf("github: parsing event file %q: %w", env.EventPath, err)
		}
		if ev.PullRequest != nil {
			env.PRNumber = ev.PullRequest.Number
			env.Description = ev.PullRequest.Body
			if env.BaseRef == "" {
				env.BaseRef = ev.PullRequest.Base.Ref
			}
			if env.HeadRef == "" {
				env.HeadRef = ev.PullRequest.Head.Ref
			}
		}
	}
	return env, nil
}

// Client posts and updates GitHub PR comments via the REST API.
type Client struct {
	apiURL string
	token  string
	hc     *http.Client

	// LogWriter receives the intended-action message in dry-run mode. When nil
	// it defaults to os.Stderr.
	LogWriter io.Writer
}

// NewClient constructs a GitHub API client. When hc is nil a client with a 30s
// timeout is used.
func NewClient(apiURL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{apiURL: apiURL, token: token, hc: hc}
}

// ghComment is one issue comment as returned by the list endpoint.
type ghComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

// UpsertComment creates or updates the single CodeSteward PR comment. It lists
// the issue comments (paginating with per_page=100 until a short page), finds
// the first whose body contains report.Marker, and PATCHes it when found or
// POSTs a new comment otherwise. In dry-run mode it logs the intended action to
// LogWriter and performs no API calls.
func (c *Client) UpsertComment(ctx context.Context, repo string, pr int, body string, dryRun bool) error {
	if dryRun {
		fmt.Fprintf(c.logDest(), "codesteward: dry-run: would create or update the CodeSteward comment on %s#%d (no GitHub API calls made)\n", repo, pr)
		return nil
	}

	commentID, found, err := c.findComment(ctx, repo, pr)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("github: encoding comment body: %w", err)
	}

	base := strings.TrimRight(c.apiURL, "/")
	if found {
		url := fmt.Sprintf("%s/repos/%s/issues/comments/%d", base, repo, commentID)
		return c.writeComment(ctx, http.MethodPatch, url, payload, repo, pr)
	}
	url := fmt.Sprintf("%s/repos/%s/issues/%d/comments", base, repo, pr)
	return c.writeComment(ctx, http.MethodPost, url, payload, repo, pr)
}

// findComment returns the id of the first existing comment whose body contains
// report.Marker, following pagination until the marker is found or a short
// (< perPage) page ends the list.
func (c *Client) findComment(ctx context.Context, repo string, pr int) (int64, bool, error) {
	const perPage = 100
	base := strings.TrimRight(c.apiURL, "/")
	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/repos/%s/issues/%d/comments?per_page=%d&page=%d", base, repo, pr, perPage, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, false, fmt.Errorf("github: building list request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.hc.Do(req)
		if err != nil {
			return 0, false, fmt.Errorf("github: listing comments: %w", err)
		}
		data, err := readResponse(resp, repo, pr)
		if err != nil {
			return 0, false, err
		}

		var comments []ghComment
		if err := json.Unmarshal(data, &comments); err != nil {
			return 0, false, fmt.Errorf("github: decoding comments: %w", err)
		}
		for _, cm := range comments {
			if strings.Contains(cm.Body, report.Marker) {
				return cm.ID, true, nil
			}
		}
		if len(comments) < perPage {
			return 0, false, nil
		}
	}
}

// writeComment issues the create/update request and maps the status code to an
// actionable error.
func (c *Client) writeComment(ctx context.Context, method, url string, payload []byte, repo string, pr int) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("github: building %s request: %w", method, err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("github: posting comment: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return statusError(resp.StatusCode, repo, pr)
}

// setHeaders applies the standard GitHub API request headers.
func (c *Client) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "codesteward")
}

// logDest resolves the dry-run log destination.
func (c *Client) logDest() io.Writer {
	if c.LogWriter != nil {
		return c.LogWriter
	}
	return os.Stderr
}

// readResponse reads and returns the body for a successful response, or maps a
// non-2xx status to an actionable error.
func readResponse(resp *http.Response, repo string, pr int) ([]byte, error) {
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github: reading response body: %w", err)
	}
	if err := statusError(resp.StatusCode, repo, pr); err != nil {
		return nil, err
	}
	return data, nil
}

// statusError converts an HTTP status code into an actionable error. 401/403
// mention the required token permission; 404 mentions the repo/PR.
func statusError(code int, repo string, pr int) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return fmt.Errorf("github: request failed with status %d: check that the token has 'pull-requests: write' permission (and 'contents: read')", code)
	case code == http.StatusNotFound:
		return fmt.Errorf("github: request failed with status 404: could not find repository %q or pull request #%d — verify they exist and the token can access them", repo, pr)
	default:
		return fmt.Errorf("github: request failed with status %d", code)
	}
}
