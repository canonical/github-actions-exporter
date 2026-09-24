package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/canonical/github-actions-exporter/internal/githubapi"
	"github.com/canonical/github-actions-exporter/internal/metrics"
)

func TestStatusToValue(t *testing.T) {
	cases := []struct {
		status, conclusion string
		want               float64
	}{
		{"completed", "success", 0},
		{"completed", "failure", 1},
		{"completed", "cancelled", 3},
		{"completed", "other", 4},
		{"in_progress", "", 2},
		{"unknown", "", 4},
	}
	for _, c := range cases {
		got := statusToValue(c.status, c.conclusion)
		if got != c.want {
			t.Errorf("statusToValue(%q, %q) = %v, want %v", c.status, c.conclusion, got, c.want)
		}
	}
}

type fakeRun struct {
	Name       string
	Status     string
	Conclusion string
	CreatedAt  string
	UpdatedAt  string
}

type fakeAPI struct{}

func (f *fakeAPI) FetchLatestWorkflowRun(_ context.Context, _ string, _ string, workflow string) (*fakeRun, error) {
	return &fakeRun{
		Name:       "TestWorkflow",
		Status:     "completed",
		Conclusion: "success",
		CreatedAt:  "2023-01-01T00:00:00Z",
		UpdatedAt:  "2023-01-01T00:01:00Z",
	}, nil
}

// This test demonstrates scrape logic with a mock, but does not check Prometheus metrics.
func TestScrapeBasic(t *testing.T) {
	// Patch githubapi.FetchLatestWorkflowRun for this test
	orig := githubapi.FetchLatestWorkflowRun
	githubapi.FetchLatestWorkflowRun = func(ctx context.Context, token, repo, workflow string) (*githubapi.WorkflowRun, error) {
		return &githubapi.WorkflowRun{
			Name:       "TestWorkflow",
			Status:     "completed",
			Conclusion: "success",
			CreatedAt:  "2023-01-01T00:00:00Z",
			UpdatedAt:  "2023-01-01T00:01:00Z",
		}, nil
	}
	defer func() { githubapi.FetchLatestWorkflowRun = orig }()

	cfg := Config{{Repo: "octocat/Hello-World", Workflows: []string{".github/workflows/ci.yaml"}}}
	// Should not panic or error
	scrape(cfg, "")
}

func TestLoadConfig(t *testing.T) {
	os.Setenv("GITHUB_ACTIONS_CONFIG", `[
	  {"repo": "octocat/Hello-World", "workflows": [".github/workflows/ci.yaml", ".github/workflows/other.yaml"]},
	  {"repo": "foo/bar", "workflows": [".github/workflows/deploy.yaml"]}
	]`)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := Config{
		{Repo: "octocat/Hello-World", Workflows: []string{".github/workflows/ci.yaml", ".github/workflows/other.yaml"}},
		{Repo: "foo/bar", Workflows: []string{".github/workflows/deploy.yaml"}},
	}
	if !reflect.DeepEqual(cfg, expected) {
		t.Errorf("got %+v, want %+v", cfg, expected)
	}
}

func TestRequestsPerHour(t *testing.T) {
	cfg := Config{
		{Repo: "canonical/service-mesh", Workflows: []string{"a.yaml", "b.yaml", "c.yaml", "d.yaml"}},
	}

	// Four workflows every 60s is 240 requests/hour, four times the unauthenticated cap of 60.
	if got := requestsPerHour(cfg, 60*time.Second); got != 240 {
		t.Errorf("requestsPerHour(4 workflows, 60s) = %v, want 240", got)
	}
	// Four minutes is the slowest poll that fits an unauthenticated budget.
	if got := requestsPerHour(cfg, 4*time.Minute); got != 60 {
		t.Errorf("requestsPerHour(4 workflows, 4m) = %v, want 60", got)
	}
}

// A failed fetch must leave the run gauges alone but still be counted, so that "the exporter
// cannot see GitHub" is distinguishable from "the workflow is fine".
func TestScrapeRecordsFailuresWithoutDisturbingTheRunGauges(t *testing.T) {
	const repo, workflow = "owner/repo", ".github/workflows/ci.yaml"

	orig := githubapi.FetchLatestWorkflowRun
	defer func() { githubapi.FetchLatestWorkflowRun = orig }()

	cfg := Config{{Repo: repo, Workflows: []string{workflow}}}

	// One good poll, so the gauges hold a known value.
	githubapi.FetchLatestWorkflowRun = func(context.Context, string, string, string) (*githubapi.WorkflowRun, error) {
		return &githubapi.WorkflowRun{
			Status: "completed", Conclusion: "success",
			CreatedAt: "2023-01-01T00:00:00Z", UpdatedAt: "2023-01-01T00:01:00Z",
		}, nil
	}
	scrape(cfg, "")

	status := metrics.RunStatus.WithLabelValues(repo, workflow)
	if v := testutil.ToFloat64(status); v != 0 {
		t.Fatalf("status after a good poll = %v, want 0", v)
	}
	lastSuccess := testutil.ToFloat64(metrics.LastScrapeSuccess.WithLabelValues(repo, workflow))
	if lastSuccess == 0 {
		t.Fatal("a successful poll must record its timestamp")
	}
	errorsBefore := testutil.ToFloat64(metrics.ScrapeErrors.WithLabelValues(repo, workflow))

	// Now a failing poll.
	githubapi.FetchLatestWorkflowRun = func(context.Context, string, string, string) (*githubapi.WorkflowRun, error) {
		return nil, errors.New("GitHub API returned status 403")
	}
	scrape(cfg, "")

	if v := testutil.ToFloat64(metrics.ScrapeErrors.WithLabelValues(repo, workflow)); v != errorsBefore+1 {
		t.Errorf("error count = %v, want %v", v, errorsBefore+1)
	}
	if v := testutil.ToFloat64(status); v != 0 {
		t.Errorf("status = %v, want the stale 0 to be preserved", v)
	}
	if v := testutil.ToFloat64(metrics.LastScrapeSuccess.WithLabelValues(repo, workflow)); v != lastSuccess {
		t.Errorf("last success = %v, want it unchanged at %v", v, lastSuccess)
	}
}
