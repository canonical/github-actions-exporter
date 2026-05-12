
package main

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/example/github-actions-exporter/internal/githubapi"
)

func TestStatusToValue(t *testing.T) {
       cases := []struct {
	       status, conclusion string
	       want float64
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
