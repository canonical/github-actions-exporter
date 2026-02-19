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
)

func Register() {
	prometheus.MustRegister(RunStatus, RunTimestamp, RunDuration)
}
