package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RunStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_action_run_status",
			Help: "Status of the most recent GitHub Actions run (0=success, 1=failure, 2=in_progress, 3=cancelled, 4=other)",
		},
		[]string{"repo", "workflow"},
	)

	RunTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_action_run_timestamp",
			Help: "Unix timestamp of the most recent GitHub Actions run",
		},
		[]string{"repo", "workflow"},
	)

	RunDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_action_run_duration_seconds",
			Help: "Duration in seconds of the most recent GitHub Actions run",
		},
		[]string{"repo", "workflow"},
	)

	// ScrapeErrors counts failed fetches. A failed fetch leaves the three gauges above at their
	// previous values, which is deliberate (a stale status is better than losing a real failure
	// alert), but it means nothing else reveals that the exporter has stopped seeing GitHub.
	ScrapeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "github_action_scrape_errors_total",
			Help: "Total number of failed attempts to fetch the most recent run of a workflow from the GitHub API",
		},
		[]string{"repo", "workflow"},
	)

	// LastScrapeSuccess is how consumers tell a fresh gauge from a stale one. It stays at 0 for a
	// workflow that has never been fetched successfully.
	LastScrapeSuccess = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "github_action_last_scrape_success_timestamp",
			Help: "Unix timestamp of the last successful fetch of this workflow from the GitHub API, 0 if it has never succeeded",
		},
		[]string{"repo", "workflow"},
	)
)

func Register() {
	prometheus.MustRegister(RunStatus, RunTimestamp, RunDuration, ScrapeErrors, LastScrapeSuccess)
}

// Init creates the scrape series for a configured repo and workflow.
//
// Labelled metrics do not exist until they are first touched, so without this a workflow whose
// very first fetch fails would be entirely absent from /metrics rather than visibly broken, and
// no alert could distinguish it from one that was never configured.
func Init(repo, workflow string) {
	ScrapeErrors.WithLabelValues(repo, workflow).Add(0)
	LastScrapeSuccess.WithLabelValues(repo, workflow).Set(0)
}
