package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// gitHubClient is a thin wrapper around the GitHub REST API.
type gitHubClient struct {
	token  string
	client *http.Client
}

func newGitHubClient(token string) *gitHubClient {
	return &gitHubClient{
		token:  token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// --- Tool handlers (each receives params map, returns result) ---

func (g *gitHubClient) listPRs(params map[string]any) (any, error) {
	owner := str(params, "owner")
	repo := str(params, "repo")
	state := strDefault(params, "state", "open")
	return g.get(fmt.Sprintf("/repos/%s/%s/pulls?state=%s&per_page=30", owner, repo, state))
}

func (g *gitHubClient) getPR(params map[string]any) (any, error) {
	owner := str(params, "owner")
	repo := str(params, "repo")
	number := num(params, "number")
	return g.get(fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number))
}

func (g *gitHubClient) getDiff(params map[string]any) (any, error) {
	owner := str(params, "owner")
	repo := str(params, "repo")
	number := num(params, "number")

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, number)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github.diff")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github %d: %s", resp.StatusCode, body)
	}
	return string(body), nil
}

func (g *gitHubClient) postComment(params map[string]any) (any, error) {
	owner := str(params, "owner")
	repo := str(params, "repo")
	number := num(params, "number")
	body := str(params, "body")

	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	payload := fmt.Sprintf(`{"body":%q}`, body)
	return g.post(path, payload)
}

func (g *gitHubClient) listCommits(params map[string]any) (any, error) {
	owner := str(params, "owner")
	repo := str(params, "repo")
	limit := numDefault(params, "limit", 20)

	path := fmt.Sprintf("/repos/%s/%s/commits?per_page=%d", owner, repo, limit)
	if branch := str(params, "branch"); branch != "" {
		path += "&sha=" + branch
	}
	if since := str(params, "since"); since != "" {
		path += "&since=" + since
	}
	return g.get(path)
}

func (g *gitHubClient) listIssues(params map[string]any) (any, error) {
	owner := str(params, "owner")
	repo := str(params, "repo")
	state := strDefault(params, "state", "open")

	path := fmt.Sprintf("/repos/%s/%s/issues?state=%s&per_page=30", owner, repo, state)
	if label := str(params, "label"); label != "" {
		path += "&labels=" + label
	}
	if assignee := str(params, "assignee"); assignee != "" {
		path += "&assignee=" + assignee
	}
	return g.get(path)
}

// --- HTTP helpers ---

func (g *gitHubClient) get(path string) (any, error) {
	return g.do("GET", path, "")
}

func (g *gitHubClient) post(path, body string) (any, error) {
	return g.do("POST", path, body)
}

func (g *gitHubClient) do(method, path, body string) (any, error) {
	url := "https://api.github.com" + path

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github %s %s: %d — %s", method, path, resp.StatusCode, data)
	}

	var result any
	json.Unmarshal(data, &result)
	return result, nil
}

// --- Param extraction helpers ---

func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func strDefault(m map[string]any, key, def string) string {
	if v := str(m, key); v != "" {
		return v
	}
	return def
}

func num(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func numDefault(m map[string]any, key string, def int) int {
	if v := num(m, key); v != 0 {
		return v
	}
	return def
}
