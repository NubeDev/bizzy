package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewWorkflowCmd creates the "nube workflow" command group.
func NewWorkflowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manage and run multi-app workflows",
		Aliases: []string{"wf"},
	}

	cmd.AddCommand(newWorkflowRunCmd())
	cmd.AddCommand(newWorkflowListCmd())
	cmd.AddCommand(newWorkflowStatusCmd())
	cmd.AddCommand(newWorkflowCancelCmd())
	cmd.AddCommand(newWorkflowDefsCmd())

	return cmd
}

func newWorkflowRunCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "run <app> <workflow> [--input key=value ...]",
		Short: "Run a workflow",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			appName := args[0]
			workflowName := args[1]

			// Parse --input flags into a map.
			inputFlags, _ := cmd.Flags().GetStringSlice("input")
			inputs := make(map[string]any)
			for _, kv := range inputFlags {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid input format %q, expected key=value", kv)
				}
				inputs[parts[0]] = parts[1]
			}

			// Generate a workflow ID.
			wfID := "wf-" + fmt.Sprintf("%d", time.Now().UnixNano())

			body := map[string]any{
				"workflow_id": wfID,
				"app":         appName,
				"workflow":    workflowName,
				"inputs":      inputs,
			}

			var result map[string]any
			status, err := client.DoJSON("POST", "/api/workflows/run", body, &result)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("server error (%d): %v", status, result["error"])
			}

			fmt.Printf("Workflow started: %s\n", wfID)
			fmt.Printf("Status: %v\n", result["status"])

			// Poll until done.
			return pollWorkflow(client, wfID, format)
		},
	}

	cmd.Flags().StringSliceP("input", "i", nil, "Input values as key=value pairs")
	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json")

	return cmd
}

func newWorkflowListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workflow runs",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			var result struct {
				Runs []struct {
					ID           string `json:"id"`
					App          string `json:"app"`
					Workflow     string `json:"workflow"`
					Status       string `json:"status"`
					CurrentStage string `json:"current_stage,omitempty"`
					CreatedAt    string `json:"created_at"`
				} `json:"runs"`
			}

			status, err := client.DoJSON("GET", "/api/workflows", nil, &result)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("server error (%d)", status)
			}

			if len(result.Runs) == 0 {
				fmt.Println("No workflow runs found.")
				return nil
			}

			for _, r := range result.Runs {
				fmt.Printf("%-20s %-20s %-20s %s\n", r.ID, r.App+"/"+r.Workflow, r.Status, r.CreatedAt)
			}
			return nil
		},
	}
}

func newWorkflowStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <workflow-id>",
		Short: "Check the status of a workflow run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			var run map[string]any
			status, err := client.DoJSON("GET", "/api/workflows/"+args[0], nil, &run)
			if err != nil {
				return err
			}
			if status == 404 {
				return fmt.Errorf("workflow run not found: %s", args[0])
			}
			if status >= 400 {
				return fmt.Errorf("server error (%d)", status)
			}

			data, _ := json.MarshalIndent(run, "", "  ")
			fmt.Println(string(data))
			return nil
		},
	}
}

func newWorkflowCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <workflow-id>",
		Short: "Cancel a running workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			var result map[string]any
			status, err := client.DoJSON("POST", "/api/workflows/"+args[0]+"/cancel", nil, &result)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("server error (%d): %v", status, result["error"])
			}

			fmt.Printf("Workflow %s cancelled.\n", args[0])
			return nil
		},
	}
}

func newWorkflowDefsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "definitions",
		Short: "List available workflow definitions",
		Aliases: []string{"defs"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			var defs []struct {
				App         string `json:"app"`
				Name        string `json:"name"`
				Description string `json:"description"`
				StageCount  int    `json:"stage_count"`
			}

			status, err := client.DoJSON("GET", "/api/workflows/definitions", nil, &defs)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("server error (%d)", status)
			}

			if len(defs) == 0 {
				fmt.Println("No workflow definitions found.")
				return nil
			}

			for _, d := range defs {
				fmt.Printf("%-20s %-25s %d stages   %s\n", d.App, d.Name, d.StageCount, d.Description)
			}
			return nil
		},
	}
}

// pollWorkflow polls a workflow run until it reaches a terminal state,
// printing stage progress as it goes.
func pollWorkflow(client *Client, wfID, format string) error {
	lastVersion := 0
	printedStages := make(map[string]bool)

	for {
		time.Sleep(1 * time.Second)

		var run struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Version int    `json:"version"`
			Error   string `json:"error,omitempty"`
			Stages  []struct {
				Name       string `json:"name"`
				Status     string `json:"status"`
				DurationMS int    `json:"duration_ms,omitempty"`
				Error      string `json:"error,omitempty"`
				Output     any    `json:"output,omitempty"`
			} `json:"stages"`
		}

		status, err := client.DoJSON("GET", "/api/workflows/"+wfID, nil, &run)
		if err != nil {
			return err
		}
		if status >= 400 {
			return fmt.Errorf("poll error (%d)", status)
		}

		// Skip if no change.
		if run.Version <= lastVersion {
			continue
		}
		lastVersion = run.Version

		// Print stage updates.
		for _, s := range run.Stages {
			key := s.Name + ":" + s.Status
			if printedStages[key] {
				continue
			}
			printedStages[key] = true

			switch s.Status {
			case "running":
				fmt.Printf("● %s — %s...\n", s.Name, s.Name)
			case "completed":
				dur := fmt.Sprintf("%.1fs", float64(s.DurationMS)/1000)
				fmt.Printf("● %s — done (%s)\n", s.Name, dur)
			case "failed":
				fmt.Printf("✗ %s — FAILED\n", s.Name)
				if s.Error != "" {
					fmt.Printf("  Error: %s\n", s.Error)
				}
			case "skipped":
				fmt.Printf("○ %s — skipped\n", s.Name)
			case "waiting":
				fmt.Printf("● %s — waiting for approval\n", s.Name)
				// For now, auto-approve in CLI (interactive approval is Phase 2).
				if s.Output != nil {
					data, _ := json.MarshalIndent(s.Output, "  ", "  ")
					fmt.Printf("  %s\n", string(data))
				}
				fmt.Print("  Approve? (y/n): ")
				var input string
				fmt.Scanln(&input)
				action := "approve"
				feedback := ""
				if strings.ToLower(input) != "y" {
					action = "reject"
					feedback = input
				}
				client.DoJSON("POST", "/api/workflows/"+wfID+"/approve", map[string]any{
					"action":   action,
					"feedback": feedback,
				}, nil)
			}
		}

		// Terminal states.
		switch run.Status {
		case "completed":
			fmt.Println("✓ Workflow completed.")
			return nil
		case "failed":
			return fmt.Errorf("workflow failed at %s: %s", run.Error, run.Error)
		case "cancelled":
			fmt.Println("Workflow cancelled.")
			return nil
		}
	}
}
