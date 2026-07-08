// Package gitlab detects the GitLab CI environment and posts/updates the
// single CodeSteward merge-request note.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/codesteward-ai/codesteward/internal/report"
)

// Env is the detected GitLab CI environment.
type Env struct {
	IsCI        bool   // true when GITLAB_CI == "true"
	ProjectID   string // CI_PROJECT_ID (numeric id or group/project path)
	MRIID       int    // CI_MERGE_REQUEST_IID
	BaseRef     string // CI_MERGE_REQUEST_TARGET_BRANCH_NAME
	HeadRef     string // CI_COMMIT_SHA, else CI_MERGE_REQUEST_SOURCE_BRANCH_NAME
	APIURL      string // CI_API_V4_URL
	Token       string // CODESTEWARD_GITLAB_TOKEN, else CI_JOB_TOKEN
	Description string // CI_MERGE_REQUEST_DESCRIPTION

	// TokenIsJobToken records that Token was sourced from CI_JOB_TOKEN rather
	// than CODESTEWARD_GITLAB_TOKEN. Job tokens usually cannot post merge
	// request notes; callers should propagate this to Client.JobToken so the
	// authentication header and 401/403 diagnostics are accurate.
	TokenIsJobToken bool
}

// DetectEnv inspects environment variables to detect a GitLab CI merge-request
// run. When GITLAB_CI is not "true" it returns an Env with IsCI == false and a
// nil error. When GitLab CI is detected but the merge-request context is
// incomplete (no MR IID, project id, or API URL) it returns the partially
// populated Env together with an actionable error.
func DetectEnv(getenv func(string) string) (*Env, error) {
	if getenv == nil {
		return &Env{}, nil
	}
	env := &Env{}
	if strings.TrimSpace(getenv("GITLAB_CI")) != "true" {
		return env, nil
	}
	env.IsCI = true
	env.ProjectID = strings.TrimSpace(getenv("CI_PROJECT_ID"))
	env.APIURL = strings.TrimSpace(getenv("CI_API_V4_URL"))
	env.BaseRef = strings.TrimSpace(getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME"))
	env.HeadRef = firstNonEmpty(getenv("CI_COMMIT_SHA"), getenv("CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"))
	env.Description = getenv("CI_MERGE_REQUEST_DESCRIPTION")

	if tok := strings.TrimSpace(getenv("CODESTEWARD_GITLAB_TOKEN")); tok != "" {
		env.Token = tok
		env.TokenIsJobToken = false
	} else if tok := strings.TrimSpace(getenv("CI_JOB_TOKEN")); tok != "" {
		env.Token = tok
		env.TokenIsJobToken = true
	}

	iidStr := strings.TrimSpace(getenv("CI_MERGE_REQUEST_IID"))
	if iidStr == "" {
		return env, fmt.Errorf("codesteward: GitLab CI detected but no merge request context (CI_MERGE_REQUEST_IID is empty); run CodeSteward in a merge request pipeline, e.g. rules: - if: '$CI_PIPELINE_SOURCE == \"merge_request_event\"'")
	}
	iid, err := strconv.Atoi(iidStr)
	if err != nil {
		return env, fmt.Errorf("codesteward: invalid CI_MERGE_REQUEST_IID %q: %w", iidStr, err)
	}
	env.MRIID = iid

	if env.ProjectID == "" {
		return env, fmt.Errorf("codesteward: GitLab CI detected but CI_PROJECT_ID is empty; cannot locate the merge request")
	}
	if env.APIURL == "" {
		return env, fmt.Errorf("codesteward: GitLab CI detected but CI_API_V4_URL is empty; cannot reach the GitLab API")
	}
	return env, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// Client posts and updates the CodeSteward GitLab merge-request note.
type Client struct {
	apiURL string
	token  string
	hc     *http.Client

	// JobToken selects the JOB-TOKEN authentication header instead of
	// PRIVATE-TOKEN. Set it when Token came from CI_JOB_TOKEN; such tokens
	// usually cannot post merge request notes, which is reflected in the
	// 401/403 error text.
	JobToken bool

	// LogWriter receives dry-run and diagnostic messages. Defaults to
	// os.Stderr when nil.
	LogWriter io.Writer
}

// NewClient constructs a GitLab API client. apiV4URL is the CI_API_V4_URL base
// (e.g. https://gitlab.com/api/v4). If hc is nil a client with a 30s timeout is
// used. All requests honor the caller's context regardless of hc.
func NewClient(apiV4URL, token string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		apiURL:    strings.TrimRight(strings.TrimSpace(apiV4URL), "/"),
		token:     token,
		hc:        hc,
		LogWriter: os.Stderr,
	}
}

// notePayload is the JSON body for creating/updating a note.
type notePayload struct {
	Body string `json:"body"`
}

// note is the subset of the GitLab note object we consume.
type note struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

// maxNotePages bounds pagination so a misbehaving API cannot loop forever.
const maxNotePages = 10000

// UpsertNote creates or updates the CodeSteward note on the merge request. It
// lists existing notes (paginated), and if it finds one whose body contains
// report.Marker it updates that note, otherwise it creates a new one. When
// dryRun is true it logs the intended action to LogWriter and makes no API
// calls.
func (c *Client) UpsertNote(ctx context.Context, projectID string, mriid int, body string, dryRun bool) error {
	if dryRun {
		fmt.Fprintf(c.logWriter(), "codesteward: dry-run: would create or update the CodeSteward note on merge request !%d in project %s; no GitLab API calls made\n", mriid, projectID)
		return nil
	}

	existingID, err := c.findNote(ctx, projectID, mriid)
	if err != nil {
		return err
	}
	if existingID != 0 {
		return c.updateNote(ctx, projectID, mriid, existingID, body)
	}
	return c.createNote(ctx, projectID, mriid, body)
}

func (c *Client) logWriter() io.Writer {
	if c.LogWriter != nil {
		return c.LogWriter
	}
	return os.Stderr
}

// notesBaseURL returns the notes collection URL, URL-escaping projectID so
// path-style ids (group/project) survive as a single path segment.
func (c *Client) notesBaseURL(projectID string, mriid int) string {
	return fmt.Sprintf("%s/projects/%s/merge_requests/%d/notes",
		c.apiURL, url.PathEscape(projectID), mriid)
}

func (c *Client) setAuth(req *http.Request) {
	if c.JobToken {
		req.Header.Set("JOB-TOKEN", c.token)
		return
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
}

// findNote returns the id of the existing CodeSteward note, or 0 if none.
func (c *Client) findNote(ctx context.Context, projectID string, mriid int) (int, error) {
	base := c.notesBaseURL(projectID, mriid)
	page := 1
	for iter := 0; iter < maxNotePages; iter++ {
		reqURL := fmt.Sprintf("%s?per_page=100&page=%d", base, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return 0, fmt.Errorf("codesteward: building gitlab list-notes request: %w", err)
		}
		c.setAuth(req)
		req.Header.Set("Accept", "application/json")

		resp, err := c.hc.Do(req)
		if err != nil {
			return 0, fmt.Errorf("codesteward: gitlab list notes request failed: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			err := c.statusError(resp, "list merge request notes")
			resp.Body.Close()
			return 0, err
		}
		var notes []note
		decErr := json.NewDecoder(resp.Body).Decode(&notes)
		nextPage := strings.TrimSpace(resp.Header.Get("X-Next-Page"))
		resp.Body.Close()
		if decErr != nil {
			return 0, fmt.Errorf("codesteward: decoding gitlab notes response: %w", decErr)
		}
		for _, n := range notes {
			if strings.Contains(n.Body, report.Marker) {
				return n.ID, nil
			}
		}
		if nextPage != "" {
			np, err := strconv.Atoi(nextPage)
			if err == nil && np > page {
				page = np
				continue
			}
			return 0, nil
		}
		// No pagination header: stop unless the page was full (length-based).
		if len(notes) < 100 {
			return 0, nil
		}
		page++
	}
	return 0, nil
}

func (c *Client) createNote(ctx context.Context, projectID string, mriid int, body string) error {
	return c.writeNote(ctx, http.MethodPost, c.notesBaseURL(projectID, mriid), body, "create merge request note")
}

func (c *Client) updateNote(ctx context.Context, projectID string, mriid, noteID int, body string) error {
	u := fmt.Sprintf("%s/%d", c.notesBaseURL(projectID, mriid), noteID)
	return c.writeNote(ctx, http.MethodPut, u, body, "update merge request note")
}

func (c *Client) writeNote(ctx context.Context, method, reqURL, body, action string) error {
	payload, err := json.Marshal(notePayload{Body: body})
	if err != nil {
		return fmt.Errorf("codesteward: encoding gitlab note body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("codesteward: building gitlab %s request: %w", action, err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("codesteward: gitlab %s request failed: %w", action, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.statusError(resp, action)
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return nil
}

// statusError converts a non-2xx response into an actionable error.
func (c *Client) statusError(resp *http.Response, action string) error {
	snippet := bodySnippet(resp.Body)
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		hint := "set CODESTEWARD_GITLAB_TOKEN to a personal, project, or group access token that has the 'api' scope and permission to comment on merge requests"
		if c.JobToken {
			hint = "the CI job token (CI_JOB_TOKEN) was used, which usually cannot post merge request notes; set CODESTEWARD_GITLAB_TOKEN to a personal, project, or group access token with the 'api' scope"
		}
		return fmt.Errorf("codesteward: gitlab API could not %s: %d %s; %s%s",
			action, resp.StatusCode, http.StatusText(resp.StatusCode), hint, snippet)
	case http.StatusNotFound:
		return fmt.Errorf("codesteward: gitlab API could not %s: 404 not found; verify the project id and merge request IID are correct and that the access token can reach this project%s",
			action, snippet)
	default:
		return fmt.Errorf("codesteward: gitlab API could not %s: %d %s%s",
			action, resp.StatusCode, http.StatusText(resp.StatusCode), snippet)
	}
}

func bodySnippet(r io.Reader) string {
	b, _ := io.ReadAll(io.LimitReader(r, 1024))
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}
