package githubapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	TotalCount   int        `json:"total_count"`
	WorkflowRuns []fakeRun  `json:"workflow_runs"`
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
