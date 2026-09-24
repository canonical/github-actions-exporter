# GitHub Actions Prometheus Exporter

A Prometheus exporter written in Go that exposes metrics about the most recent runs of specified GitHub Actions workflows.

## Features
- Scrapes the status, timestamp, and duration of the most recent run for multiple workflows across multiple repositories.
- Exposes metrics for Prometheus at `/metrics`.
- Configurable via environment variables.

## Configuration


Set the following environment variables:

  - `GITHUB_ACTIONS_CONFIG`: JSON array specifying repos and workflows to monitor. Example:
    ```json
    [
      {
        "repo": "owner/repo",
        "workflows": [".github/workflows/ci.yaml", ".github/workflows/other.yaml"]
      }
    ]
    ```
- `GITHUB_TOKEN`: (Optional, but see below) GitHub personal access token for private repos or higher rate limits.

### Rate limits

Each poll costs one API request per monitored workflow, so the configuration above costs
`workflows x 3600/SCRAPE_INTERVAL` requests per hour.

Without `GITHUB_TOKEN`, GitHub allows **60 requests per hour per source address**. Exceed that and
every remaining request that hour is answered with HTTP 403, the run gauges freeze at their last
values, and `github_action_scrape_errors_total` starts climbing. Four workflows at the default 60s
interval already needs 240 requests/hour, so a token is effectively required for anything but the
smallest configuration. Any token lifts the limit to 5000 requests/hour — for public repositories
one with no scopes and no permissions at all is enough.

The exporter logs the computed hourly request count at startup when no token is set.
- `EXPORTER_PORT`: (Optional) Port to serve metrics (default: 8080).
- `SCRAPE_INTERVAL`: (Optional) How often to fetch fresh data from the GitHub API, e.g. `60s` (default: 60s). This is independent of how often Prometheus scrapes the `/metrics` endpoint. The exporter continuously fetches workflow data from GitHub at this interval and caches the results.

## Proxy Support

To use a network proxy for GitHub API requests, set the standard `HTTP_PROXY`, `HTTPS_PROXY`, or `NO_PROXY` environment variables:

```
HTTP_PROXY="http://proxy.example.com:8080" GITHUB_ACTIONS_CONFIG='[{"repo": "owner/repo", "workflows": [".github/workflows/ci.yaml"]}]' ./github-actions-exporter
```

The exporter will use these standard environment variables for outbound GitHub API requests.

## Metrics
- `github_action_run_status{repo, workflow}`: Status of the most recent run (0=success, 1=failure, 2=in_progress, 3=cancelled, 4=other). The `workflow` label is the workflow file path (e.g., `.github/workflows/ci.yaml`).
- `github_action_run_timestamp{repo, workflow}`: Unix timestamp of the most recent run.
- `github_action_run_duration_seconds{repo, workflow}`: Duration in seconds of the most recent run.
- `github_action_scrape_errors_total{repo, workflow}`: Counter of failed attempts to fetch this workflow from the GitHub API.
- `github_action_last_scrape_success_timestamp{repo, workflow}`: Unix timestamp of the last successful fetch, or 0 if it has never succeeded.

A failed fetch deliberately leaves the three `run_*` gauges at their previous values, because
dropping the series would silently resolve a genuine workflow-failure alert. That means those
gauges can be stale, and the only way to know is
`github_action_scrape_errors_total` and `github_action_last_scrape_success_timestamp`. Alert on
them — a rate-limited exporter otherwise looks perfectly healthy:

```promql
# The exporter has not managed to reach GitHub for this workflow in a while.
increase(github_action_scrape_errors_total[10m]) > 0
```

## Building

```
go build -o github-actions-exporter ./cmd
```

## Running

```
GITHUB_ACTIONS_CONFIG='[{"repo": "octocat/Hello-World", "workflows": [".github/workflows/ci.yaml"]}]' ./github-actions-exporter
```

## License
See [LICENSE](LICENSE).
