package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codesteward-ai/codesteward/internal/report"
)

// mapGetenv builds a getenv func backed by a map.
func mapGetenv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDetectEnv(t *testing.T) {
	prEvent, err := filepath.Abs(filepath.Join("testdata", "event_pull_request.json"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	pushEvent, err := filepath.Abs(filepath.Join("testdata", "event_push.json"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    Env
		wantErr bool
	}{
		{
			name: "full pull_request event",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "codesteward-ai/demo",
				"GITHUB_API_URL":    "https://ghe.example.com/api/v3",
				"GITHUB_TOKEN":      "tok",
				"GITHUB_EVENT_PATH": prEvent,
			},
			want: Env{
				IsActions:   true,
				Repo:        "codesteward-ai/demo",
				PRNumber:    7,
				BaseRef:     "main",
				HeadRef:     "feature/cache",
				EventPath:   prEvent,
				APIURL:      "https://ghe.example.com/api/v3",
				Token:       "tok",
				Description: "This PR adds a small cache to the runtime.\n\nMotivation: reduce repeated work.",
			},
		},
		{
			name: "api url defaults when unset",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "o/r",
				"GITHUB_EVENT_PATH": prEvent,
			},
			want: Env{
				IsActions:   true,
				Repo:        "o/r",
				PRNumber:    7,
				BaseRef:     "main",
				HeadRef:     "feature/cache",
				EventPath:   prEvent,
				APIURL:      defaultAPIURL,
				Description: "This PR adds a small cache to the runtime.\n\nMotivation: reduce repeated work.",
			},
		},
		{
			name: "env refs win over event refs",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "o/r",
				"GITHUB_BASE_REF":   "release",
				"GITHUB_HEAD_REF":   "topic",
				"GITHUB_EVENT_PATH": prEvent,
			},
			want: Env{
				IsActions:   true,
				Repo:        "o/r",
				PRNumber:    7,
				BaseRef:     "release",
				HeadRef:     "topic",
				EventPath:   prEvent,
				APIURL:      defaultAPIURL,
				Description: "This PR adds a small cache to the runtime.\n\nMotivation: reduce repeated work.",
			},
		},
		{
			name: "push event has no pull request",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_REPOSITORY": "o/r",
				"GITHUB_EVENT_PATH": pushEvent,
			},
			want: Env{
				IsActions: true,
				Repo:      "o/r",
				EventPath: pushEvent,
				APIURL:    defaultAPIURL,
			},
		},
		{
			name: "not github actions",
			env:  map[string]string{},
			want: Env{
				APIURL: defaultAPIURL,
			},
		},
		{
			name: "missing event file is an error",
			env: map[string]string{
				"GITHUB_ACTIONS":    "true",
				"GITHUB_EVENT_PATH": filepath.Join(t.TempDir(), "does-not-exist.json"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectEnv(mapGetenv(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (env=%+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if *got != tt.want {
				t.Fatalf("DetectEnv mismatch:\n got  %+v\n want %+v", *got, tt.want)
			}
		})
	}
}

func TestDetectEnvInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := DetectEnv(mapGetenv(map[string]string{
		"GITHUB_ACTIONS":    "true",
		"GITHUB_EVENT_PATH": bad,
	}))
	if err == nil {
		t.Fatal("expected error for malformed event JSON")
	}
	if !strings.Contains(err.Error(), "parsing event file") {
		t.Fatalf("error should mention parsing: %v", err)
	}
}

// recordingHandler captures requests for assertions.
type capture struct {
	method string
	path   string
	query  string
	body   string
}

func TestUpsertCommentCreateNew(t *testing.T) {
	var got []capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, capture{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		switch r.Method {
		case http.MethodGet:
			// No existing comments.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok", srv.Client())
	body := report.Marker + "\n\nhello"
	if err := client.UpsertComment(context.Background(), "owner/repo", 7, body, false); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 requests (list + create), got %d: %+v", len(got), got)
	}
	if got[0].method != http.MethodGet || got[0].path != "/repos/owner/repo/issues/7/comments" {
		t.Errorf("list request wrong: %+v", got[0])
	}
	if !strings.Contains(got[0].query, "per_page=100") || !strings.Contains(got[0].query, "page=1") {
		t.Errorf("list query missing pagination params: %q", got[0].query)
	}
	if got[1].method != http.MethodPost || got[1].path != "/repos/owner/repo/issues/7/comments" {
		t.Errorf("create request wrong: %+v", got[1])
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(got[1].body), &payload); err != nil {
		t.Fatalf("create body not json: %v (%q)", err, got[1].body)
	}
	if payload["body"] != body {
		t.Errorf("create body mismatch: %q", payload["body"])
	}
}

func TestUpsertCommentUpdateExisting(t *testing.T) {
	var got []capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, capture{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			resp := []ghComment{
				{ID: 10, Body: "unrelated comment"},
				{ID: 42, Body: "prefix " + report.Marker + " suffix"},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":42}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok", srv.Client())
	body := report.Marker + "\nupdated"
	if err := client.UpsertComment(context.Background(), "owner/repo", 7, body, false); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 requests (list + patch), got %d: %+v", len(got), got)
	}
	if got[1].method != http.MethodPatch || got[1].path != "/repos/owner/repo/issues/comments/42" {
		t.Errorf("patch request wrong: %+v", got[1])
	}
}

func TestUpsertCommentMarkerOnPageTwo(t *testing.T) {
	var got []capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, capture{r.Method, r.URL.Path, r.URL.RawQuery, string(b)})
		switch r.Method {
		case http.MethodGet:
			page := r.URL.Query().Get("page")
			w.Header().Set("Content-Type", "application/json")
			switch page {
			case "1":
				// A full page of 100 comments, none with the marker.
				var full []ghComment
				for i := 0; i < 100; i++ {
					full = append(full, ghComment{ID: int64(i + 1), Body: fmt.Sprintf("noise %d", i)})
				}
				_ = json.NewEncoder(w).Encode(full)
			case "2":
				_ = json.NewEncoder(w).Encode([]ghComment{{ID: 555, Body: report.Marker}})
			default:
				t.Errorf("unexpected page %q", page)
				_, _ = w.Write([]byte("[]"))
			}
		case http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":555}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok", srv.Client())
	if err := client.UpsertComment(context.Background(), "owner/repo", 7, report.Marker+"\nx", false); err != nil {
		t.Fatalf("UpsertComment: %v", err)
	}

	// Expect: GET page 1, GET page 2, PATCH.
	if len(got) != 3 {
		t.Fatalf("expected 3 requests, got %d: %+v", len(got), got)
	}
	if !strings.Contains(got[0].query, "page=1") {
		t.Errorf("first request not page 1: %q", got[0].query)
	}
	if !strings.Contains(got[1].query, "page=2") {
		t.Errorf("second request not page 2: %q", got[1].query)
	}
	if got[2].method != http.MethodPatch || got[2].path != "/repos/owner/repo/issues/comments/555" {
		t.Errorf("patch request wrong: %+v", got[2])
	}
}

func TestUpsertCommentUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "bad", srv.Client())
	err := client.UpsertComment(context.Background(), "owner/repo", 7, "body", false)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "pull-requests: write") {
		t.Fatalf("401 error should mention token permission, got: %v", err)
	}
}

func TestUpsertCommentForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok", srv.Client())
	err := client.UpsertComment(context.Background(), "owner/repo", 7, "body", false)
	if err == nil || !strings.Contains(err.Error(), "pull-requests: write") {
		t.Fatalf("403 error should mention token permission, got: %v", err)
	}
}

func TestUpsertCommentNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok", srv.Client())
	err := client.UpsertComment(context.Background(), "owner/repo", 7, "body", false)
	if err == nil {
		t.Fatal("expected error on 404")
	}
	if !strings.Contains(err.Error(), "owner/repo") || !strings.Contains(err.Error(), "#7") {
		t.Fatalf("404 error should mention repo and PR, got: %v", err)
	}
}

func TestUpsertCommentDryRunZeroRequests(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Errorf("dry-run must not make any HTTP request, got %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	var log bytes.Buffer
	client := NewClient(srv.URL, "tok", srv.Client())
	client.LogWriter = &log

	if err := client.UpsertComment(context.Background(), "owner/repo", 7, "body", true); err != nil {
		t.Fatalf("dry-run should not error: %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected zero requests in dry-run, got %d", requests)
	}
	if !strings.Contains(log.String(), "dry-run") || !strings.Contains(log.String(), "owner/repo#7") {
		t.Fatalf("dry-run log missing expected content: %q", log.String())
	}
}

func TestUpsertCommentContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	client := NewClient(srv.URL, "tok", srv.Client())
	err := client.UpsertComment(ctx, "owner/repo", 7, "body", false)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context cancellation error, got: %v", err)
	}
}

func TestNewClientDefaultsTimeout(t *testing.T) {
	c := NewClient(defaultAPIURL, "tok", nil)
	if c.hc == nil {
		t.Fatal("expected non-nil default http client")
	}
	if c.hc.Timeout != 30*time.Second {
		t.Fatalf("expected 30s default timeout, got %v", c.hc.Timeout)
	}
}
