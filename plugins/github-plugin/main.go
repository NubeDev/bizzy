// github-plugin provides GitHub tools to bizzy apps via the plugin SDK.
//
// Tools: list_prs, get_pr, get_diff, post_comment, list_commits, list_issues
//
// Usage:
//
//	export GITHUB_TOKEN=ghp_...
//	go run .
package main

import (
	"log"
	"os"

	"github.com/NubeDev/bizzy/pkg/pluginsdk"
)

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN is required")
	}

	gh := newGitHubClient(token)

	p := pluginsdk.NewPlugin("github", "1.0.0", "GitHub integration — PRs, commits, issues, and code review")
	p.SetPreamble("Use plugin.github.* tools to interact with GitHub repositories.")

	// --- list_prs ---
	schema := pluginsdk.Params("owner", "string", "Repository owner", true)
	pluginsdk.ParamsAdd(schema, "repo", "string", "Repository name", true)
	pluginsdk.ParamsAdd(schema, "state", "string", "Filter: open, closed, all (default: open)", false)
	p.AddTool(pluginsdk.Tool{
		Name:        "list_prs",
		Description: "List pull requests for a repository",
		Parameters:  schema,
		Handler:     gh.listPRs,
	})

	// --- get_pr ---
	schema = pluginsdk.Params("owner", "string", "Repository owner", true)
	pluginsdk.ParamsAdd(schema, "repo", "string", "Repository name", true)
	pluginsdk.ParamsAdd(schema, "number", "number", "PR number", true)
	p.AddTool(pluginsdk.Tool{
		Name:        "get_pr",
		Description: "Get details of a specific pull request",
		Parameters:  schema,
		Handler:     gh.getPR,
	})

	// --- get_diff ---
	schema = pluginsdk.Params("owner", "string", "Repository owner", true)
	pluginsdk.ParamsAdd(schema, "repo", "string", "Repository name", true)
	pluginsdk.ParamsAdd(schema, "number", "number", "PR number", true)
	p.AddTool(pluginsdk.Tool{
		Name:        "get_diff",
		Description: "Get the unified diff of a pull request",
		Parameters:  schema,
		Handler:     gh.getDiff,
	})

	// --- post_comment ---
	schema = pluginsdk.Params("owner", "string", "Repository owner", true)
	pluginsdk.ParamsAdd(schema, "repo", "string", "Repository name", true)
	pluginsdk.ParamsAdd(schema, "number", "number", "PR or issue number", true)
	pluginsdk.ParamsAdd(schema, "body", "string", "Comment body (markdown)", true)
	p.AddTool(pluginsdk.Tool{
		Name:        "post_comment",
		Description: "Post a comment on a PR or issue",
		Parameters:  schema,
		Handler:     gh.postComment,
	})

	// --- list_commits ---
	schema = pluginsdk.Params("owner", "string", "Repository owner", true)
	pluginsdk.ParamsAdd(schema, "repo", "string", "Repository name", true)
	pluginsdk.ParamsAdd(schema, "branch", "string", "Branch name (default: default branch)", false)
	pluginsdk.ParamsAdd(schema, "since", "string", "ISO 8601 date to filter from", false)
	pluginsdk.ParamsAdd(schema, "limit", "number", "Max commits to return (default: 20)", false)
	p.AddTool(pluginsdk.Tool{
		Name:        "list_commits",
		Description: "List recent commits on a branch",
		Parameters:  schema,
		Handler:     gh.listCommits,
	})

	// --- list_issues ---
	schema = pluginsdk.Params("owner", "string", "Repository owner", true)
	pluginsdk.ParamsAdd(schema, "repo", "string", "Repository name", true)
	pluginsdk.ParamsAdd(schema, "state", "string", "Filter: open, closed, all (default: open)", false)
	pluginsdk.ParamsAdd(schema, "label", "string", "Filter by label", false)
	pluginsdk.ParamsAdd(schema, "assignee", "string", "Filter by assignee", false)
	p.AddTool(pluginsdk.Tool{
		Name:        "list_issues",
		Description: "List issues for a repository",
		Parameters:  schema,
		Handler:     gh.listIssues,
	})

	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
