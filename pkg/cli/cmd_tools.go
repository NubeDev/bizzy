package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// NewToolsCmd creates the "nube tools" command — lists the user's installed callable tools.
func NewToolsCmd() *cobra.Command {
	var appFilter string

	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List your installed tools",
		Long: `Shows all callable tools from your installed apps.
Prompt-mode tools are excluded — use "nube prompts" to see those.

Examples:
  nube tools
  nube tools --app rubix
  nube tools -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			status, data, err := client.Do("GET", "/my/tools", nil)
			if err != nil {
				return err
			}
			CheckError(status, data)

			var tools []toolEntry
			json.Unmarshal(data, &tools)

			// Filter out prompt-mode tools (they belong in "nube prompts").
			var callable []toolEntry
			for _, t := range tools {
				if t.Mode == "prompt" {
					continue
				}
				if appFilter != "" && t.AppName != appFilter {
					continue
				}
				callable = append(callable, t)
			}

			sort.Slice(callable, func(i, j int) bool {
				return callable[i].Name < callable[j].Name
			})

			if IsJSON() {
				PrintJSON(callable)
				return nil
			}

			if len(callable) == 0 {
				if appFilter != "" {
					fmt.Printf("No tools found for app %q\n", appFilter)
				} else {
					fmt.Println("No tools installed. Browse the store: nube store browse")
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "NAME\tAPP\tTYPE\tDESCRIPTION\n")
			for _, t := range callable {
				desc := t.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Name, t.AppName, t.Type, desc)
			}
			w.Flush()
			fmt.Fprintf(os.Stderr, "\n%d tools. Run with: nube run <name> [--param key=val]\n", len(callable))
			return nil
		},
	}

	cmd.Flags().StringVar(&appFilter, "app", "", "Filter by app name")
	return cmd
}

// NewPromptsCmd creates the "nube prompts" command — lists prompts and prompt-mode tools.
func NewPromptsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompts [name]",
		Short: "List your available prompts",
		Long: `Shows all prompts from your installed apps, including prompt-mode tools.

Without arguments: lists all prompts.
With a name: shows detail view with arguments and options.

Examples:
  nube prompts
  nube prompts rubix.navigation
  nube prompts -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				return showPromptDetail(client, args[0])
			}
			return listAllPrompts(client)
		},
	}
	return cmd
}

func listAllPrompts(client *Client) error {
	// Fetch both prompts and tools, merge prompt-mode tools into the list.
	var prompts []promptEntry
	var tools []toolEntry

	status, data, err := client.Do("GET", "/my/prompts", nil)
	if err != nil {
		return err
	}
	CheckError(status, data)
	json.Unmarshal(data, &prompts)

	status, data, err = client.Do("GET", "/my/tools", nil)
	if err != nil {
		return err
	}
	CheckError(status, data)
	json.Unmarshal(data, &tools)

	type promptRow struct {
		Name    string `json:"name"`
		AppName string `json:"app"`
		Desc    string `json:"description"`
		Options string `json:"options,omitempty"`
		Source  string `json:"source"` // "prompt" or "tool"
	}

	var rows []promptRow

	// Add markdown prompts.
	for _, p := range prompts {
		rows = append(rows, promptRow{
			Name:    p.Name,
			AppName: p.AppName,
			Desc:    p.Description,
			Source:  "prompt",
		})
	}

	// Add prompt-mode tools (these don't appear in /my/prompts).
	seen := make(map[string]bool)
	for _, r := range rows {
		seen[r.Name] = true
	}
	for _, t := range tools {
		if t.Mode != "prompt" {
			continue
		}
		if seen[t.Name] {
			continue
		}
		var opts []string
		for _, p := range t.Params {
			opts = append(opts, p.Options...)
		}
		rows = append(rows, promptRow{
			Name:    t.Name,
			AppName: t.AppName,
			Desc:    t.Description,
			Options: strings.Join(opts, ", "),
			Source:  "tool",
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	if IsJSON() {
		PrintJSON(rows)
		return nil
	}

	if len(rows) == 0 {
		fmt.Println("No prompts installed. Browse the store: nube store browse")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tAPP\tDESCRIPTION\tOPTIONS\n")
	for _, r := range rows {
		desc := r.Desc
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		opts := r.Options
		if len(opts) > 30 {
			opts = opts[:27] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.AppName, desc, opts)
	}
	w.Flush()
	fmt.Fprintf(os.Stderr, "\n%d prompts. Use with: nube ask \"/name [option]\"\n", len(rows))
	return nil
}

func showPromptDetail(client *Client, name string) error {
	// First check if it's a prompt-mode tool (has options, params).
	status, data, err := client.Do("GET", "/my/tools", nil)
	if err != nil {
		return err
	}
	CheckError(status, data)

	var tools []toolEntry
	json.Unmarshal(data, &tools)

	for _, t := range tools {
		if t.Name == name && t.Mode == "prompt" {
			if IsJSON() {
				PrintJSON(t)
				return nil
			}
			fmt.Printf("Name:        %s\n", t.Name)
			fmt.Printf("App:         %s\n", t.AppName)
			fmt.Printf("Type:        prompt\n")
			fmt.Printf("Description: %s\n", t.Description)
			if len(t.Params) > 0 {
				fmt.Println("\nArguments:")
				for _, p := range t.Params {
					req := ""
					if p.Required {
						req = " (required)"
					}
					fmt.Printf("  %-12s %s%s\n", p.Name, p.Description, req)
					if len(p.Options) > 0 {
						fmt.Printf("  %-12s options: %s\n", "", strings.Join(p.Options, " | "))
					}
				}
			}
			fmt.Println("\nUsage:")
			if len(t.Params) > 0 && len(t.Params[0].Options) > 0 {
				for _, opt := range t.Params[0].Options {
					fmt.Printf("  nube ask \"/%s %s\"\n", name, opt)
				}
			} else {
				fmt.Printf("  nube ask \"/%s\"\n", name)
			}
			return nil
		}
	}

	// Fall back to checking markdown prompts.
	status, data, err = client.Do("GET", "/my/prompts", nil)
	if err != nil {
		return err
	}
	CheckError(status, data)

	var prompts []promptEntry
	json.Unmarshal(data, &prompts)

	for _, p := range prompts {
		if p.Name == name {
			if IsJSON() {
				PrintJSON(p)
				return nil
			}
			fmt.Printf("Name:        %s\n", p.Name)
			fmt.Printf("App:         %s\n", p.AppName)
			fmt.Printf("Type:        prompt\n")
			fmt.Printf("Description: %s\n", p.Description)
			if len(p.Arguments) > 0 {
				fmt.Println("\nArguments:")
				for _, a := range p.Arguments {
					req := ""
					if a.Required {
						req = " (required)"
					}
					fmt.Printf("  %-12s%s\n", a.Name, req)
				}
			}
			fmt.Printf("\nUsage:\n  nube ask \"/%s\"\n", name)
			return nil
		}
	}

	return fmt.Errorf("prompt not found: %s\nRun 'nube prompts' to see available prompts", name)
}

// NewProvidersCmd creates the "nube providers" command — top-level provider listing.
func NewProvidersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "providers [name]",
		Short: "List AI providers or test one",
		Long: `Shows available AI providers and their status.
With a name argument, tests connectivity to that provider.

Examples:
  nube providers
  nube providers test ollama
  nube providers -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				// Test a specific provider.
				status, data, err := client.Do("POST", "/api/agents/providers/"+args[0]+"/test", nil)
				if err != nil {
					return err
				}
				CheckError(status, data)
				if IsJSON() {
					PrintRawJSON(data)
					return nil
				}
				var result struct {
					Provider  string   `json:"provider"`
					Available bool     `json:"available"`
					Error     string   `json:"error"`
					LatencyMS int      `json:"latency_ms"`
					Models    []string `json:"models"`
				}
				json.Unmarshal(data, &result)
				avail := "yes"
				if !result.Available {
					avail = "no"
				}
				fmt.Printf("Provider:  %s\n", result.Provider)
				fmt.Printf("Available: %s\n", avail)
				fmt.Printf("Latency:   %dms\n", result.LatencyMS)
				if len(result.Models) > 0 {
					fmt.Printf("Models:    %s\n", strings.Join(result.Models, ", "))
				}
				if result.Error != "" {
					fmt.Printf("Error:     %s\n", result.Error)
				}
				return nil
			}

			// List all providers.
			status, data, err := client.Do("GET", "/api/agents/providers", nil)
			if err != nil {
				return err
			}
			CheckError(status, data)

			if IsJSON() {
				PrintRawJSON(data)
				return nil
			}

			var providers []struct {
				Provider  string   `json:"provider"`
				Available bool     `json:"available"`
				Type      string   `json:"type"`
				Models    []string `json:"models"`
			}
			json.Unmarshal(data, &providers)

			sort.Slice(providers, func(i, j int) bool {
				return providers[i].Provider < providers[j].Provider
			})

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "PROVIDER\tAVAILABLE\tTYPE\tMODELS\n")
			for _, p := range providers {
				avail := "yes"
				if !p.Available {
					avail := "no"
					_ = avail
				}
				models := strings.Join(p.Models, ", ")
				if models == "" {
					models = "-"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Provider, avail, p.Type, models)
			}
			w.Flush()
			return nil
		},
	}
}

// --- shared types for JSON unmarshaling ---

type toolEntry struct {
	Name        string      `json:"name"`
	AppName     string      `json:"appName"`
	Type        string      `json:"type"`
	Mode        string      `json:"mode,omitempty"`
	Prompt      string      `json:"prompt,omitempty"`
	Description string      `json:"description"`
	Params      []paramEntry `json:"params,omitempty"`
}

type paramEntry struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Options     []string `json:"options,omitempty"`
}

type promptEntry struct {
	Name        string `json:"name"`
	AppName     string `json:"appName"`
	Description string `json:"description"`
	Arguments   []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	} `json:"arguments,omitempty"`
}
