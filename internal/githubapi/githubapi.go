package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// apiBaseURL allows test injection of the GitHub API base URL
var apiBaseURL = "https://api.github.com"

// requestTimeout bounds a single GitHub API call. Without it, a connection that is accepted but
// never answered would block the scrape loop indefinitely, freezing every metric at its last
// value while logging nothing at all.
const requestTimeout = 30 * time.Second

// maxErrorBodyBytes caps how much of a failure response is kept for the error message. GitHub's
// explanations are a short JSON object, so this is plenty.
const maxErrorBodyBytes = 512

type WorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	RunNumber  int    `json:"run_number"`
	RunAttempt int    `json:"run_attempt"`
	WorkflowID int64  `json:"workflow_id"`
	HTMLURL    string `json:"html_url"`
}

type WorkflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

// APIError is returned when GitHub answers with a non-200 status.
//
// It exists because the interesting failure modes are indistinguishable from the status code
// alone: an exhausted rate limit, a fine-grained token without access to the repository, a token
// that has not been authorised for the organisation's SAML SSO, and an IP allow list rejection
// all arrive as HTTP 403. GitHub explains which one it is in the response body and the
// x-ratelimit-* headers, so those are carried here rather than discarded.
type APIError struct {
	StatusCode int
	// Body is the response body, truncated to maxErrorBodyBytes.
	Body string
	// RateLimitRemaining is the x-ratelimit-remaining header, empty if absent.
	RateLimitRemaining string
	// RateLimitReset is the x-ratelimit-reset header, zero if absent or unparseable.
	RateLimitReset time.Time
	// RetryAfter is the retry-after header, set by GitHub for secondary rate limits.
	RetryAfter string
}

// RateLimited reports whether the request was rejected for exceeding a rate limit rather than
// for a permissions reason.
func (e *APIError) RateLimited() bool {
	if e.StatusCode != http.StatusForbidden && e.StatusCode != http.StatusTooManyRequests {
		return false
	}
	return e.RateLimitRemaining == "0" || e.RetryAfter != ""
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "GitHub API returned status %d", e.StatusCode)

	if e.RateLimited() {
		b.WriteString(" (rate limit exceeded")
		if !e.RateLimitReset.IsZero() {
			fmt.Fprintf(&b, ", resets at %s", e.RateLimitReset.Format(time.RFC3339))
		}
		if e.RetryAfter != "" {
			fmt.Fprintf(&b, ", retry-after %ss", e.RetryAfter)
		}
		b.WriteString(")")
	} else if e.StatusCode == http.StatusForbidden {
		// Not a rate limit, so the token is the problem. Name the usual suspects, because the
		// body alone ("Resource not accessible by personal access token") rarely says which.
		b.WriteString(" (not a rate limit: check that the token has Actions read access to this" +
			" repository, and that it is authorised for the organisation's SSO)")
	}

	if e.Body != "" {
		fmt.Fprintf(&b, ": %s", e.Body)
	}
	return b.String()
}

// newAPIError builds an APIError from a failed response, consuming its body.
func newAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))

	err := &APIError{
		StatusCode:         resp.StatusCode,
		Body:               strings.TrimSpace(string(body)),
		RateLimitRemaining: resp.Header.Get("X-RateLimit-Remaining"),
		RetryAfter:         resp.Header.Get("Retry-After"),
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if seconds, convErr := strconv.ParseInt(reset, 10, 64); convErr == nil {
			err.RateLimitReset = time.Unix(seconds, 0).UTC()
		}
	}
	return err
}

// FetchLatestWorkflowRun fetches the latest run for a given repo and workflow file path
var FetchLatestWorkflowRun = func(ctx context.Context, token, repo, workflowFilePath string) (*WorkflowRun, error) {
	encodedPath := url.PathEscape(workflowFilePath)
	apiURL := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/runs?per_page=1", apiBaseURL, repo, encodedPath)

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	client := &http.Client{Transport: transport, Timeout: requestTimeout}

	request, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, newAPIError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var runsResp WorkflowRunsResponse
	if err := json.Unmarshal(body, &runsResp); err != nil {
		return nil, err
	}
	if len(runsResp.WorkflowRuns) == 0 {
		return nil, fmt.Errorf("no workflow runs found")
	}
	return &runsResp.WorkflowRuns[0], nil
}
