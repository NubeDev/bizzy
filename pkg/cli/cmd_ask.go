package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/NubeDev/bizzy/pkg/claude"
	"github.com/NubeDev/bizzy/pkg/models"
	"github.com/spf13/cobra"
)

// NewAskCmd creates the "nube ask" command that calls Claude Code.
func NewAskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask <prompt>",
		Short: "Ask Claude a question with access to your Nube tools",
		Long: `Send a prompt to Claude Code with your Nube MCP server connected.

Claude will have access to all your installed apps and tools
(e.g. nube-marketing, rubix-developer).

Examples:
  nube ask "write a marketing plan for Rubix"
  nube ask "list all devices and their status"
  nube ask "review this content for our website"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := strings.Join(args, " ")

			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			if cfg.Server == "" {
				return fmt.Errorf("not logged in — run: nube login <server-url> <token>")
			}

			mcpURL := strings.TrimRight(cfg.Server, "/") + "/mcp"
			sessionID := models.GenerateID("ses-")

			var lastWasText bool

			claude.Run(claude.RunConfig{
				Prompt:       prompt,
				MCPURL:       mcpURL,
				MCPToken:     cfg.Token,
				AllowedTools: "mcp__nube__*",
			}, sessionID, func(ev claude.Event) {
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
				}
			})

			return nil
		},
	}
}
