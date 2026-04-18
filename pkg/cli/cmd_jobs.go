package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// NewJobsCmd creates the "nube jobs" command group.
func NewJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage async AI jobs (submit, poll, list, cancel)",
	}

	cmd.AddCommand(newJobsSubmitCmd())
	cmd.AddCommand(newJobsPollCmd())
	cmd.AddCommand(newJobsListCmd())
	cmd.AddCommand(newJobsCancelCmd())

	return cmd
}

func newJobsSubmitCmd() *cobra.Command {
	var provider, model, agent string

	cmd := &cobra.Command{
		Use:   "submit <prompt>",
		Short: "Submit an async AI job",
		Long: `Submit a prompt as a background job. Returns a job ID immediately.
Use "nube jobs poll <id>" to stream results or "nube jobs cancel <id>" to stop it.

Examples:
  nube jobs submit "generate weekly report"
  nube jobs submit --provider ollama --model gemma3 "check devices"
  JOB=$(nube jobs submit -o json "report" | jq -r .job_id)`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")
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

	cmd.Flags().StringVar(&provider, "provider", "", "AI provider (default: your preference)")
	cmd.Flags().StringVar(&model, "model", "", "Model override")
	cmd.Flags().StringVar(&agent, "agent", "", "Agent/app name")

	return cmd
}

func newJobsPollCmd() *cobra.Command {
	var once bool

	cmd := &cobra.Command{
		Use:   "poll <job-id>",
		Short: "Poll a job for status and events",
		Long: `Stream events from a running job until it completes.
Use --once to check once and exit.

Examples:
  nube jobs poll job-abc123
  nube jobs poll job-abc123 --once
  nube jobs poll job-abc123 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			if once {
				return pollOnce(client, args[0])
			}
			return pollUntilDone(client, args[0])
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "Check once and exit")
	return cmd
}

func newJobsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active and recent jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			status, data, err := client.Do("GET", "/api/agents/jobs", nil)
			if err != nil {
				return err
			}
			CheckError(status, data)

			if IsJSON() {
				PrintRawJSON(data)
				return nil
			}

			var jobs []struct {
				JobID    string `json:"job_id"`
				Status   string `json:"status"`
				Provider string `json:"provider"`
				Model    string `json:"model"`
			}
			json.Unmarshal(data, &jobs)

			if len(jobs) == 0 {
				fmt.Println("No active jobs.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "JOB ID\tSTATUS\tPROVIDER\tMODEL\n")
			for _, j := range jobs {
				m := j.Model
				if m == "" {
					m = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", j.JobID, j.Status, j.Provider, m)
			}
			w.Flush()
			return nil
		},
	}
}

func newJobsCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <job-id>",
		Short: "Cancel a running job",
		Long: `Stop a running AI job. The underlying AI process is killed immediately.

Examples:
  nube jobs cancel job-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			status, data, err := client.Do("DELETE", "/api/agents/jobs/"+args[0], nil)
			if err != nil {
				return err
			}
			CheckError(status, data)

			fmt.Printf("Job %s cancelled.\n", args[0])
			return nil
		},
	}
}

// pollOnce and pollUntilDone are defined in cmd_agents.go and reused here.
