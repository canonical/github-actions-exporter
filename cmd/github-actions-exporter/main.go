package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/example/github-actions-exporter/internal/githubapi"
	"github.com/example/github-actions-exporter/internal/metrics"
)
type WorkflowConfig struct {
	Repo      string   `json:"repo"`
	Workflows []string `json:"workflows"` // workflow file paths
}

type Config []WorkflowConfig

func loadConfig() (Config, error) {
	var cfg Config
	jsonStr := os.Getenv("GITHUB_ACTIONS_CONFIG")
	if jsonStr == "" {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func statusToValue(status, conclusion string) float64 {
	switch status {
	case "completed":
		switch conclusion {
		case "success":
			return 0
		case "failure":
			return 1
		case "cancelled":
			return 3
		default:
			return 4
		}
	case "in_progress":
		return 2
	default:
		return 4
	}
}

func scrape(cfg Config, token string) {
	ctx := context.Background()
	for _, repoCfg := range cfg {
		       for _, workflowPath := range repoCfg.Workflows {
			       run, err := githubapi.FetchLatestWorkflowRun(ctx, token, repoCfg.Repo, workflowPath)
			       if err != nil {
				       log.Printf("error fetching workflow file %s for repo %s: %v", workflowPath, repoCfg.Repo, err)
				       continue
			       }
		       labels := []string{repoCfg.Repo, workflowPath}
			       metrics.RunStatus.WithLabelValues(labels...).Set(statusToValue(run.Status, run.Conclusion))
			       t, _ := time.Parse(time.RFC3339, run.CreatedAt)
			       metrics.RunTimestamp.WithLabelValues(labels...).Set(float64(t.Unix()))
			       if run.CreatedAt != "" && run.UpdatedAt != "" {
				start, _ := time.Parse(time.RFC3339, run.CreatedAt)
				end, _ := time.Parse(time.RFC3339, run.UpdatedAt)
				dur := end.Sub(start).Seconds()
				metrics.RunDuration.WithLabelValues(labels...).Set(dur)
			}
		}
	}
}

func main() {
	log.Println("Starting GitHub Actions Prometheus Exporter...")
	metrics.Register()
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	if cfg == nil {
		log.Fatalf("GITHUB_ACTIONS_CONFIG is not set or empty")
	}
	token := os.Getenv("GITHUB_TOKEN")
	interval := 60 * time.Second
	if v := os.Getenv("SCRAPE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}
	go func() {
		for {
			scrape(cfg, token)
			time.Sleep(interval)
		}
	}()
	port := os.Getenv("EXPORTER_PORT")
	if port == "" {
		port = "8080"
	}
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on :%s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
