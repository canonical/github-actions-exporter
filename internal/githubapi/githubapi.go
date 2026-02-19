package githubapi


import (
       "context"
       "encoding/json"
       "fmt"
       "io/ioutil"
       "net/http"
       "net/url"
)

// apiBaseURL allows test injection of the GitHub API base URL
var apiBaseURL = "https://api.github.com"

type WorkflowRun struct {
       ID        int64  `json:"id"`
       Name      string `json:"name"`
       Status    string `json:"status"`
       Conclusion string `json:"conclusion"`
       CreatedAt string `json:"created_at"`
       UpdatedAt string `json:"updated_at"`
       RunNumber int    `json:"run_number"`
       RunAttempt int   `json:"run_attempt"`
       WorkflowID int64 `json:"workflow_id"`
       HTMLURL   string `json:"html_url"`
}

type WorkflowRunsResponse struct {
       TotalCount   int           `json:"total_count"`
       WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

// FetchLatestWorkflowRun fetches the latest run for a given repo and workflow file path
var FetchLatestWorkflowRun = func(ctx context.Context, token, repo, workflowFilePath string) (*WorkflowRun, error) {
	encodedPath := url.PathEscape(workflowFilePath)
	apiURL := fmt.Sprintf("%s/repos/%s/actions/workflows/%s/runs?per_page=1", apiBaseURL, repo, encodedPath)

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	client := &http.Client{Transport: transport}

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

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
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
