package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codesteward-ai/codesteward/internal/report"
)

const markerBody = report.Marker + "\n\n## CodeSteward: Ready for maintainer review\n"

func mapGetenv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDetectEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantErr     bool
		wantIsCI    bool
		wantProject string
		wantIID     int
		wantBase    string
		wantHead    string
		wantToken   string
		wantJobTok  bool
		wantDesc    string
	}{
		{
			name:     "not gitlab ci",
			env:      map[string]string{},
			wantErr:  false,
			wantIsCI: false,
		},
		{
			name: "gitlab ci false string",
			env: map[string]string{
				"GITLAB_CI": "false",
			},
			wantIsCI: false,
		},
		{
			name: "full mr env personal token",
			env: map[string]string{
				"GITLAB_CI":                           "true",
				"CI_PROJECT_ID":                       "42",
				"CI_MERGE_REQUEST_IID":                "7",
				"CI_API_V4_URL":                       "https://gitlab.com/api/v4",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "main",
				"CI_COMMIT_SHA":                       "deadbeef",
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature",
				"CI_MERGE_REQUEST_DESCRIPTION":        "Fixes things",
				"CODESTEWARD_GITLAB_TOKEN":            "glpat-secret",
				"CI_JOB_TOKEN":                        "jobtok",
			},
			wantIsCI:    true,
			wantProject: "42",
			wantIID:     7,
			wantBase:    "main",
			wantHead:    "deadbeef",
			wantToken:   "glpat-secret",
			wantJobTok:  false,
			wantDesc:    "Fixes things",
		},
		{
			name: "falls back to job token",
			env: map[string]string{
				"GITLAB_CI":            "true",
				"CI_PROJECT_ID":        "group/project",
				"CI_MERGE_REQUEST_IID": "3",
				"CI_API_V4_URL":        "https://gitlab.example.com/api/v4",
				"CI_JOB_TOKEN":         "jobtok",
			},
			wantIsCI:    true,
			wantProject: "group/project",
			wantIID:     3,
			wantToken:   "jobtok",
			wantJobTok:  true,
		},
		{
			name: "head falls back to source branch when no sha",
			env: map[string]string{
				"GITLAB_CI":                           "true",
				"CI_PROJECT_ID":                       "1",
				"CI_MERGE_REQUEST_IID":                "9",
				"CI_API_V4_URL":                       "https://gitlab.com/api/v4",
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "feature",
				"CODESTEWARD_GITLAB_TOKEN":            "t",
			},
			wantIsCI:    true,
			wantProject: "1",
			wantIID:     9,
			wantHead:    "feature",
			wantToken:   "t",
		},
		{
			name: "missing mr iid is an error",
			env: map[string]string{
				"GITLAB_CI":     "true",
				"CI_PROJECT_ID": "42",
				"CI_API_V4_URL": "https://gitlab.com/api/v4",
			},
			wantErr:  true,
			wantIsCI: true,
		},
		{
			name: "non numeric iid is an error",
			env: map[string]string{
				"GITLAB_CI":            "true",
				"CI_PROJECT_ID":        "42",
				"CI_MERGE_REQUEST_IID": "notanumber",
				"CI_API_V4_URL":        "https://gitlab.com/api/v4",
			},
			wantErr:  true,
			wantIsCI: true,
		},
		{
			name: "missing project id is an error",
			env: map[string]string{
				"GITLAB_CI":            "true",
				"CI_MERGE_REQUEST_IID": "7",
				"CI_API_V4_URL":        "https://gitlab.com/api/v4",
			},
			wantErr:  true,
			wantIsCI: true,
		},
		{
			name: "missing api url is an error",
			env: map[string]string{
				"GITLAB_CI":            "true",
				"CI_PROJECT_ID":        "42",
				"CI_MERGE_REQUEST_IID": "7",
			},
			wantErr:  true,
			wantIsCI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := DetectEnv(mapGetenv(tt.env))
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil (env=%+v)", env)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env == nil {
				t.Fatalf("expected non-nil env")
			}
			if env.IsCI != tt.wantIsCI {
				t.Errorf("IsCI = %v, want %v", env.IsCI, tt.wantIsCI)
			}
			if tt.wantErr {
				return
			}
			if env.ProjectID != tt.wantProject {
				t.Errorf("ProjectID = %q, want %q", env.ProjectID, tt.wantProject)
			}
			if env.MRIID != tt.wantIID {
				t.Errorf("MRIID = %d, want %d", env.MRIID, tt.wantIID)
			}
			if env.BaseRef != tt.wantBase {
				t.Errorf("BaseRef = %q, want %q", env.BaseRef, tt.wantBase)
			}
			if env.HeadRef != tt.wantHead {
				t.Errorf("HeadRef = %q, want %q", env.HeadRef, tt.wantHead)
			}
			if env.Token != tt.wantToken {
				t.Errorf("Token = %q, want %q", env.Token, tt.wantToken)
			}
			if env.TokenIsJobToken != tt.wantJobTok {
				t.Errorf("TokenIsJobToken = %v, want %v", env.TokenIsJobToken, tt.wantJobTok)
			}
			if env.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", env.Description, tt.wantDesc)
			}
		})
	}
}

func TestDetectEnvNilGetenv(t *testing.T) {
	env, err := DetectEnv(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env == nil || env.IsCI {
		t.Fatalf("expected non-CI env, got %+v", env)
	}
}

// recordingServer captures each request method and raw request URI.
type recordedReq struct {
	Method     string
	RequestURI string
	Path       string
	PrivateHdr string
	JobHdr     string
	Body       string
}

func TestUpsertNoteCreateNew(t *testing.T) {
	var reqs []recordedReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqs = append(reqs, recordedReq{
			Method:     r.Method,
			RequestURI: r.RequestURI,
			Path:       r.URL.Path,
			PrivateHdr: r.Header.Get("PRIVATE-TOKEN"),
			JobHdr:     r.Header.Get("JOB-TOKEN"),
			Body:       string(body),
		})
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, "[]")
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":123,"body":"created"}`)
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "glpat-xyz", srv.Client())
	if err := c.UpsertNote(context.Background(), "42", 7, markerBody, false); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}

	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests (list + create), got %d: %+v", len(reqs), reqs)
	}
	if reqs[0].Method != http.MethodGet {
		t.Errorf("first request = %s, want GET", reqs[0].Method)
	}
	if !strings.Contains(reqs[0].RequestURI, "per_page=100") {
		t.Errorf("list request missing per_page=100: %s", reqs[0].RequestURI)
	}
	if reqs[1].Method != http.MethodPost {
		t.Errorf("second request = %s, want POST", reqs[1].Method)
	}
	if reqs[1].PrivateHdr != "glpat-xyz" {
		t.Errorf("PRIVATE-TOKEN header = %q, want glpat-xyz", reqs[1].PrivateHdr)
	}
	if reqs[1].JobHdr != "" {
		t.Errorf("JOB-TOKEN header set unexpectedly: %q", reqs[1].JobHdr)
	}
	var payload notePayload
	if err := json.Unmarshal([]byte(reqs[1].Body), &payload); err != nil {
		t.Fatalf("create body not JSON: %v (%s)", err, reqs[1].Body)
	}
	if !strings.Contains(payload.Body, report.Marker) {
		t.Errorf("create body missing marker: %q", payload.Body)
	}
	if !strings.HasSuffix(reqs[1].Path, "/merge_requests/7/notes") {
		t.Errorf("create path = %q, want suffix /merge_requests/7/notes", reqs[1].Path)
	}
}

func TestUpsertNoteUpdateExisting(t *testing.T) {
	var reqs []recordedReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqs = append(reqs, recordedReq{Method: r.Method, RequestURI: r.RequestURI, Path: r.URL.Path, Body: string(body)})
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			notes := []note{
				{ID: 1, Body: "unrelated comment"},
				{ID: 55, Body: markerBody},
			}
			json.NewEncoder(w).Encode(notes)
		case http.MethodPut:
			io.WriteString(w, `{"id":55,"body":"updated"}`)
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", srv.Client())
	if err := c.UpsertNote(context.Background(), "42", 7, "new body "+report.Marker, false); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests (list + put), got %d: %+v", len(reqs), reqs)
	}
	if reqs[1].Method != http.MethodPut {
		t.Errorf("second request = %s, want PUT", reqs[1].Method)
	}
	if !strings.HasSuffix(reqs[1].Path, "/notes/55") {
		t.Errorf("update path = %q, want suffix /notes/55", reqs[1].Path)
	}
}

func TestUpsertNotePagination(t *testing.T) {
	var listPages []string
	var putCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			page := r.URL.Query().Get("page")
			listPages = append(listPages, page)
			w.Header().Set("Content-Type", "application/json")
			switch page {
			case "1":
				var full []note
				for i := 0; i < 100; i++ {
					full = append(full, note{ID: i + 1, Body: fmt.Sprintf("comment %d", i)})
				}
				w.Header().Set("X-Next-Page", "2")
				json.NewEncoder(w).Encode(full)
			case "2":
				json.NewEncoder(w).Encode([]note{{ID: 999, Body: markerBody}})
			default:
				t.Errorf("unexpected page %q", page)
				io.WriteString(w, "[]")
			}
		case http.MethodPut:
			putCount++
			if !strings.HasSuffix(r.URL.Path, "/notes/999") {
				t.Errorf("update path = %q, want suffix /notes/999", r.URL.Path)
			}
			io.WriteString(w, `{"id":999}`)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", srv.Client())
	if err := c.UpsertNote(context.Background(), "42", 7, markerBody, false); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}
	if len(listPages) != 2 || listPages[0] != "1" || listPages[1] != "2" {
		t.Errorf("expected to fetch pages [1 2], got %v", listPages)
	}
	if putCount != 1 {
		t.Errorf("expected exactly 1 PUT, got %d", putCount)
	}
}

func TestUpsertNoteUnauthorizedText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"message":"401 Unauthorized"}`)
	}))
	defer srv.Close()

	t.Run("personal token", func(t *testing.T) {
		c := NewClient(srv.URL, "tok", srv.Client())
		err := c.UpsertNote(context.Background(), "42", 7, markerBody, false)
		if err == nil {
			t.Fatal("expected error on 401")
		}
		msg := err.Error()
		for _, want := range []string{"401", "api", "access token"} {
			if !strings.Contains(msg, want) {
				t.Errorf("401 error %q missing %q", msg, want)
			}
		}
		if strings.Contains(msg, "job token") {
			t.Errorf("personal-token 401 should not mention job token: %q", msg)
		}
	})

	t.Run("job token", func(t *testing.T) {
		c := NewClient(srv.URL, "jobtok", srv.Client())
		c.JobToken = true
		err := c.UpsertNote(context.Background(), "42", 7, markerBody, false)
		if err == nil {
			t.Fatal("expected error on 401")
		}
		msg := err.Error()
		for _, want := range []string{"401", "job token", "api"} {
			if !strings.Contains(msg, want) {
				t.Errorf("job-token 401 error %q missing %q", msg, want)
			}
		}
		if req := lastAuthHeader(t, srv, c); req != "JOB-TOKEN" {
			t.Errorf("expected JOB-TOKEN auth header, got %s", req)
		}
	})
}

// lastAuthHeader issues a fresh request against a header-capturing server to
// confirm the client uses the expected auth header for the given client.
func lastAuthHeader(t *testing.T, _ *httptest.Server, c *Client) string {
	t.Helper()
	var authKind string
	hdrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("JOB-TOKEN") != "" {
			authKind = "JOB-TOKEN"
		} else if r.Header.Get("PRIVATE-TOKEN") != "" {
			authKind = "PRIVATE-TOKEN"
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, "[]")
	}))
	defer hdrSrv.Close()
	probe := NewClient(hdrSrv.URL, c.token, hdrSrv.Client())
	probe.JobToken = c.JobToken
	_ = probe.UpsertNote(context.Background(), "1", 1, "x", false)
	return authKind
}

func TestUpsertNoteNotFoundText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"message":"404 Project Not Found"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", srv.Client())
	err := c.UpsertNote(context.Background(), "42", 7, markerBody, false)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	msg := err.Error()
	for _, want := range []string{"404", "project id", "merge request IID"} {
		if !strings.Contains(msg, want) {
			t.Errorf("404 error %q missing %q", msg, want)
		}
	}
}

func TestUpsertNoteDryRunMakesNoRequests(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient(srv.URL, "tok", srv.Client())
	c.LogWriter = &buf
	if err := c.UpsertNote(context.Background(), "group/project", 7, markerBody, true); err != nil {
		t.Fatalf("dry-run UpsertNote: %v", err)
	}
	if count != 0 {
		t.Errorf("dry-run made %d HTTP requests, want 0", count)
	}
	log := buf.String()
	if !strings.Contains(log, "dry-run") {
		t.Errorf("dry-run log %q missing 'dry-run'", log)
	}
	if !strings.Contains(log, "!7") {
		t.Errorf("dry-run log %q missing MR reference", log)
	}
	if !strings.Contains(log, "group/project") {
		t.Errorf("dry-run log %q missing project", log)
	}
}

func TestUpsertNoteEscapesProjectID(t *testing.T) {
	var listURI, writeURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listURI = r.RequestURI
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, "[]")
		case http.MethodPost:
			writeURI = r.RequestURI
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"id":1}`)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", srv.Client())
	if err := c.UpsertNote(context.Background(), "my-group/sub/project", 7, markerBody, false); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}
	const wantEsc = "my-group%2Fsub%2Fproject"
	if !strings.Contains(listURI, wantEsc) {
		t.Errorf("list URI %q missing escaped project id %q", listURI, wantEsc)
	}
	if !strings.Contains(writeURI, wantEsc) {
		t.Errorf("create URI %q missing escaped project id %q", writeURI, wantEsc)
	}
	if strings.Contains(listURI, "my-group/sub/project") {
		t.Errorf("list URI %q leaked unescaped project path", listURI)
	}
}

func TestUpsertNoteHonorsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, "[]")
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	c := NewClient(srv.URL, "tok", srv.Client())
	err := c.UpsertNote(ctx, "42", 7, markerBody, false)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error %q does not reflect context cancellation", err.Error())
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("https://gitlab.com/api/v4/", "tok", nil)
	if c.hc == nil {
		t.Fatal("expected default http client")
	}
	if c.hc.Timeout == 0 {
		t.Error("expected default client timeout to be set")
	}
	if c.LogWriter == nil {
		t.Error("expected default LogWriter")
	}
	if strings.HasSuffix(c.apiURL, "/") {
		t.Errorf("apiURL should have trailing slash trimmed, got %q", c.apiURL)
	}
}
