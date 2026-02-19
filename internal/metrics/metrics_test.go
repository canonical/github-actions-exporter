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
