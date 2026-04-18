# 04 — Tool Implementations

`plugins/github-plugin/tools.go` — GitHub API client only, no NATS.

The client is a thin wrapper around the GitHub REST API. Each method maps 1:1 to a tool in the manifest.

```go
type GitHubClient struct {
    token  string
    client *http.Client
}

func newGitHubClient(token string) *GitHubClient {
    return &GitHubClient{
        token:  token,
        client: &http.Client{Timeout: 15 * time.Second},
    }
}

// do makes an authenticated GET request and returns the raw body.
func (g *GitHubClient) do(method, path string) ([]byte, error) {
    req, err := http.NewRequest(method, "https://api.github.com"+path, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+g.token)
    req.Header.Set("Accept", "application/vnd.github+json")
    req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

    resp, err := g.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("github api %s: %d — %s", path, resp.StatusCode, body)
    }
    return body, nil
}

func (g *GitHubClient) ListPRs(owner, repo, state string) (any, error) {
    if state == "" {
        state = "open"
    }
    body, err := g.do("GET", fmt.Sprintf("/repos/%s/%s/pulls?state=%s&per_page=30", owner, repo, state))
    if err != nil {
        return nil, err
    }
    var result any
    json.Unmarshal(body, &result)
    return result, nil
}

func (g *GitHubClient) GetPR(owner, repo string, number int) (any, error) {
    body, err := g.do("GET", fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number))
    if err != nil {
        return nil, err
    }
    var result any
    json.Unmarshal(body, &result)
    return result, nil
}

// GetDiff uses the GitHub diff media type to return a unified diff string.
func (g *GitHubClient) GetDiff(owner, repo string, number int) (string, error) {
    req, _ := http.NewRequest("GET",
        fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, number), nil)
    req.Header.Set("Authorization", "Bearer "+g.token)
    req.Header.Set("Accept", "application/vnd.github.diff")

    resp, err := g.client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return string(body), nil
}

func (g *GitHubClient) ListCommits(owner, repo, branch, since string, limit int) (any, error) {
    if limit == 0 {
        limit = 20
    }
    path := fmt.Sprintf("/repos/%s/%s/commits?per_page=%d", owner, repo, limit)
    if branch != "" {
        path += "&sha=" + branch
    }
    if since != "" {
        path += "&since=" + since
    }
    body, err := g.do("GET", path)
    if err != nil {
        return nil, err
    }
    var result any
    json.Unmarshal(body, &result)
    return result, nil
}

func (g *GitHubClient) ListIssues(owner, repo, state, label, assignee string) (any, error) {
    if state == "" {
        state = "open"
    }
    path := fmt.Sprintf("/repos/%s/%s/issues?state=%s&per_page=30", owner, repo, state)
    if label != "" {
        path += "&labels=" + label
    }
    if assignee != "" {
        path += "&assignee=" + assignee
    }
    body, err := g.do("GET", path)
    if err != nil {
        return nil, err
    }
    var result any
    json.Unmarshal(body, &result)
    return result, nil
}
```
