package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type fakeRunsResp struct {
	TotalCount   int       `json:"total_count"`
	WorkflowRuns []fakeRun `json:"workflow_runs"`
}

func TestFetchLatestWorkflowRun_WithProxy(t *testing.T) {
	resp := fakeRunsResp{
		TotalCount: 1,
		WorkflowRuns: []fakeRun{{
			ID: 2, Name: "ProxyTest", Status: "completed", Conclusion: "success", CreatedAt: "2023-01-02T00:00:00Z", UpdatedAt: "2023-01-02T00:01:00Z",
		}},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// Set up a proxy that just forwards to the test server
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Scheme = "http"
		r.URL.Host = ts.Listener.Addr().String()
		http.DefaultTransport.RoundTrip(r)
	}))
	defer proxy.Close()

	old := apiBaseURL
	apiBaseURL = ts.URL
	defer func() { apiBaseURL = old }()

	// Set the standard proxy environment variable
	t.Setenv("HTTP_PROXY", proxy.URL)

	run, err := FetchLatestWorkflowRun(context.Background(), "", "owner/repo", ".github/workflows/ci.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Name != "ProxyTest" || run.Status != "completed" || run.Conclusion != "success" {
		t.Errorf("unexpected run: %+v", run)
	}
}

// serveError stands up a server that answers every request with the given status, headers and
// body, and points the package at it.
func serveError(t *testing.T, status int, headers map[string]string, body string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)

	old := apiBaseURL
	apiBaseURL = ts.URL
	t.Cleanup(func() { apiBaseURL = old })
}

func fetch(t *testing.T) error {
	t.Helper()
	_, err := FetchLatestWorkflowRun(context.Background(), "", "owner/repo", ".github/workflows/ci.yaml")
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

// An exhausted rate limit and a token without access both arrive as 403. The error has to say
// which, otherwise the only way to tell them apart is guesswork.
func TestAPIError_RateLimitIsNamed(t *testing.T) {
	reset := time.Now().Add(42 * time.Minute).Truncate(time.Second)
	serveError(t, http.StatusForbidden, map[string]string{
		"X-RateLimit-Remaining": "0",
		"X-RateLimit-Reset":     strconv.FormatInt(reset.Unix(), 10),
	}, `{"message":"API rate limit exceeded for 1.2.3.4."}`)

	err := fetch(t)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %T", err)
	}
	if !apiErr.RateLimited() {
		t.Error("a 403 with x-ratelimit-remaining: 0 must be reported as rate limited")
	}
	if !apiErr.RateLimitReset.Equal(reset.UTC()) {
		t.Errorf("reset = %v, want %v", apiErr.RateLimitReset, reset.UTC())
	}

	msg := err.Error()
	for _, want := range []string{"rate limit exceeded", reset.UTC().Format(time.RFC3339), "1.2.3.4"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not mention %q", msg, want)
		}
	}
}

func TestAPIError_ForbiddenWithoutRateLimitPointsAtTheToken(t *testing.T) {
	serveError(t, http.StatusForbidden, map[string]string{
		"X-RateLimit-Remaining": "4987",
	}, `{"message":"Resource not accessible by personal access token"}`)

	err := fetch(t)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %T", err)
	}
	if apiErr.RateLimited() {
		t.Error("a 403 with quota remaining is not a rate limit")
	}

	msg := err.Error()
	if strings.Contains(msg, "rate limit exceeded") {
		t.Errorf("error %q must not blame the rate limit", msg)
	}
	for _, want := range []string{"not a rate limit", "SSO", "Resource not accessible"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not mention %q", msg, want)
		}
	}
}

// GitHub uses retry-after rather than x-ratelimit-remaining for secondary rate limits.
func TestAPIError_SecondaryRateLimit(t *testing.T) {
	serveError(t, http.StatusTooManyRequests, map[string]string{"Retry-After": "60"}, "slow down")

	err := fetch(t)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %T", err)
	}
	if !apiErr.RateLimited() {
		t.Error("a 429 with retry-after must be reported as rate limited")
	}
	if !strings.Contains(err.Error(), "retry-after 60s") {
		t.Errorf("error %q does not carry retry-after", err.Error())
	}
}

func TestAPIError_BodyIsTruncated(t *testing.T) {
	serveError(t, http.StatusInternalServerError, nil, strings.Repeat("x", maxErrorBodyBytes*3))

	err := fetch(t)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected an *APIError, got %T", err)
	}
	if len(apiErr.Body) != maxErrorBodyBytes {
		t.Errorf("body length = %d, want %d", len(apiErr.Body), maxErrorBodyBytes)
	}
}

// A hung GitHub connection must not stall the scrape loop forever.
func TestFetchLatestWorkflowRun_HasATimeout(t *testing.T) {
	if requestTimeout <= 0 {
		t.Fatal("requestTimeout must be positive, otherwise a hung call blocks the scrape loop")
	}

	blocked := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	defer func() { close(blocked); ts.Close() }()

	old := apiBaseURL
	apiBaseURL = ts.URL
	defer func() { apiBaseURL = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := FetchLatestWorkflowRun(ctx, "", "owner/repo", ".github/workflows/ci.yaml")
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the request to be cut short")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request was not bounded")
	}
}
