package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRunStatusMetric(t *testing.T) {
	Register()
	RunStatus.WithLabelValues("repo", "workflow").Set(1)
	if v := testutil.ToFloat64(RunStatus.WithLabelValues("repo", "workflow")); v != 1 {
		t.Errorf("expected 1, got %v", v)
	}
}

// A workflow whose very first fetch fails must still be present on /metrics, otherwise it cannot
// be told apart from one that was never configured.
func TestInitCreatesTheScrapeSeries(t *testing.T) {
	Init("owner/never-worked", ".github/workflows/ci.yaml")

	errs := ScrapeErrors.WithLabelValues("owner/never-worked", ".github/workflows/ci.yaml")
	if v := testutil.ToFloat64(errs); v != 0 {
		t.Errorf("error count = %v, want 0", v)
	}

	last := LastScrapeSuccess.WithLabelValues("owner/never-worked", ".github/workflows/ci.yaml")
	if v := testutil.ToFloat64(last); v != 0 {
		t.Errorf("last success = %v, want 0 to mean never", v)
	}
}
