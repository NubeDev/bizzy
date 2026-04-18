package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// RegisterAgentsCmds adds the hand-written submit/poll subcommands to an
// existing "agents" parent command (typically auto-generated from the OpenAPI spec).
// If no "agents" group exists yet, it creates one.
func RegisterAgentsCmds(root *cobra.Command) {
	// Find existing "agents" group (created by OpenAPI auto-generation).
	var agents *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "agents" {
			agents = c
			break
		}
	}
	if agents == nil {
		agents = &cobra.Command{
			Use:   "agents",
			Short: "Manage AI agents and async jobs",
		}
		root.AddCommand(agents)
	}

	agents.AddCommand(newSubmitCmd())
	agents.AddCommand(newPollCmd())
}

func newSubmitCmd() *cobra.Command {
	var provider, model, agent string

	cmd := &cobra.Command{
		Use:   "submit <prompt>",
		Short: "Submit an async AI job (returns job ID immediately)",
		Long: `Submit a prompt as an async job. The server runs the AI in the background.
Use "nube agents poll <job-id>" to check progress.

Examples:
  nube agents submit "generate weekly report"
  nube agents submit --provider ollama --model gemma3 "check devices"
  JOB=$(nube agents submit -o json "generate report" | jq -r .job_id)`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := ""
			for i, a := range args {
				if i > 0 {
					prompt += " "
				}
				prompt += a
			}

			client, err := NewClient()
			if err != nil {
				return err
			}

			body := map[string]string{"prompt": prompt}
			if provider != "" {
				body["provider"] = provider
			}
			if model != "" {
				body["model"] = model
			}
			if agent != "" {
				body["agent"] = agent
			}

			status, data, err := client.Do("POST", "/api/agents/jobs", body)
			if err != nil {
				return err
			}
			CheckError(status, data)

			if IsJSON() {
				PrintRawJSON(data)
			} else {
				var resp struct {
					JobID  string `json:"job_id"`
					Status string `json:"status"`
				}
				json.Unmarshal(data, &resp)
				fmt.Printf("Job submitted: %s (status: %s)\n", resp.JobID, resp.Status)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (default: server default)")
	cmd.Flags().StringVar(&model, "model", "", "Model override")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent/app name")

	return cmd
}

func newPollCmd() *cobra.Command {
	var once bool

	cmd := &cobra.Command{
		Use:   "poll <job-id>",
		Short: "Poll a job for status and events",
		Long: `Poll a running job until it completes, streaming events to the terminal.
Use --once to check once and exit.

Examples:
  nube agents poll job-abc123
  nube agents poll job-abc123 --once
  nube agents poll job-abc123 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]

			client, err := NewClient()
			if err != nil {
				return err
			}

			if once {
				return pollOnce(client, jobID)
			}
			return pollUntilDone(client, jobID)
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "Check once and exit (don't wait for completion)")

	return cmd
}

func pollOnce(client *Client, jobID string) error {
	status, data, err := client.Do("GET", "/api/agents/jobs/"+jobID, nil)
	if err != nil {
		return err
	}
	CheckError(status, data)

	if IsJSON() {
		PrintRawJSON(data)
		return nil
	}

	var job struct {
		JobID    string `json:"job_id"`
		Status   string `json:"status"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Result   string `json:"result"`
		Events   []struct {
			Type    string `json:"type"`
			Content string `json:"content"`
			Name    string `json:"name"`
		} `json:"events"`
	}
	json.Unmarshal(data, &job)

	fmt.Fprintf(os.Stderr, "Job: %s  Status: %s  Provider: %s\n", job.JobID, job.Status, job.Provider)
	if job.Result != "" {
		fmt.Println(job.Result)
	}
	return nil
}

func pollUntilDone(client *Client, jobID string) error {
	after := -1
	var lastWasText bool

	for {
		path := fmt.Sprintf("/api/agents/jobs/%s?after=%d", jobID, after)
		status, data, err := client.Do("GET", path, nil)
		if err != nil {
			return err
		}
		CheckError(status, data)

		var job struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
			Events []struct {
				Index      int     `json:"index"`
				Type       string  `json:"type"`
				Model      string  `json:"model"`
				Name       string  `json:"name"`
				Content    string  `json:"content"`
				Error      string  `json:"error"`
				DurationMS int     `json:"duration_ms"`
				CostUSD    float64 `json:"cost_usd"`
			} `json:"events"`
		}
		json.Unmarshal(data, &job)

		for _, ev := range job.Events {
			after = ev.Index
			switch ev.Type {
			case "connected":
				fmt.Fprintf(os.Stderr, "\033[2m⚡ Connected — model: %s\033[0m\n\n", ev.Model)
			case "tool_call":
				if lastWasText {
					fmt.Println()
					lastWasText = false
				}
				fmt.Fprintf(os.Stderr, "\033[36m⚙ calling %s\033[0m\n", ev.Name)
			case "text":
				fmt.Print(ev.Content)
				lastWasText = true
			case "error":
				fmt.Fprintf(os.Stderr, "\033[31m✗ %s\033[0m\n", ev.Error)
			case "done":
				if lastWasText {
					fmt.Println()
				}
				fmt.Fprintf(os.Stderr, "\n\033[2m— done (%dms, $%.4f)\033[0m\n", ev.DurationMS, ev.CostUSD)
				return nil
			}
		}

		if job.Status == "done" || job.Status == "error" || job.Status == "cancelled" {
			if lastWasText {
				fmt.Println()
			}
			if job.Status != "done" {
				fmt.Fprintf(os.Stderr, "\n\033[2m— %s\033[0m\n", job.Status)
			}
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}
