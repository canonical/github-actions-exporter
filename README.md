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
- `GITHUB_TOKEN`: (Optional) GitHub personal access token for private repos or higher rate limits.
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
