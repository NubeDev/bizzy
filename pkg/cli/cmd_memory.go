package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NewMemoryCmd creates the "nube memory" command tree.
//
//	nube memory                           # show both server + user memory
//	nube memory server                    # show server memory
//	nube memory server set "new content"  # replace server memory (admin)
//	nube memory me                        # show my memory
//	nube memory me set "new content"      # replace my memory
//	nube memory me add "I prefer Celsius" # append to my memory
func NewMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "View and manage AI memory",
		Long: `View and manage persistent AI memory (server-wide and per-user).

Memory is prepended to every AI conversation so the AI starts
each session already knowing what it learned before.

Examples:
  nube memory                           # show all memory
  nube memory server                    # show server memory
  nube memory server set "We use Celsius"
  nube memory me                        # show my memory
  nube memory me set "I prefer detailed responses"
  nube memory me add "My team manages floors 5-8"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			return showAllMemory(client)
		},
	}

	cmd.AddCommand(newMemoryServerCmd())
	cmd.AddCommand(newMemoryMeCmd())

	return cmd
}

func showAllMemory(client *Client) error {
	type memBody struct {
		Content string `json:"content"`
	}

	var server, user memBody

	status, data, err := client.Do("GET", "/api/memory/server", nil)
	if err != nil {
		return err
	}
	// Non-admin may get 403 — that's OK, just skip server memory.
	if status == 200 {
		json.Unmarshal(data, &server)
	}

	status, data, err = client.Do("GET", "/api/memory/me", nil)
	if err != nil {
		return err
	}
	CheckError(status, data)
	json.Unmarshal(data, &user)

	if IsJSON() {
		PrintJSON(map[string]string{
			"server": server.Content,
			"user":   user.Content,
		})
		return nil
	}

	if server.Content == "" && user.Content == "" {
		fmt.Println("No memory stored yet.")
		fmt.Println("\nSet server memory:  nube memory server set \"...\"")
		fmt.Println("Set your memory:    nube memory me set \"...\"")
		return nil
	}

	if server.Content != "" {
		fmt.Println("=== Server Memory ===")
		fmt.Println(server.Content)
		fmt.Println()
	}
	if user.Content != "" {
		fmt.Println("=== My Memory ===")
		fmt.Println(user.Content)
	}
	return nil
}

func newMemoryServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "View or manage server-wide memory (admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			status, data, err := client.Do("GET", "/api/memory/server", nil)
			if err != nil {
				return err
			}
			CheckError(status, data)

			var body struct {
				Content string `json:"content"`
			}
			json.Unmarshal(data, &body)

			if IsJSON() {
				PrintJSON(body)
				return nil
			}

			if body.Content == "" {
				fmt.Println("No server memory set.")
				fmt.Println("Set with: nube memory server set \"...\"")
				return nil
			}
			fmt.Println(body.Content)
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "set <content>",
		Short: "Replace server memory (admin)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			content := strings.Join(args, " ")
			status, data, err := client.Do("PUT", "/api/memory/server", map[string]string{"content": content})
			if err != nil {
				return err
			}
			CheckError(status, data)
			fmt.Println("Server memory updated.")
			return nil
		},
	})

	return cmd
}

func newMemoryMeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "View or manage your personal memory",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			status, data, err := client.Do("GET", "/api/memory/me", nil)
			if err != nil {
				return err
			}
			CheckError(status, data)

			var body struct {
				Content string `json:"content"`
			}
			json.Unmarshal(data, &body)

			if IsJSON() {
				PrintJSON(body)
				return nil
			}

			if body.Content == "" {
				fmt.Println("No personal memory set.")
				fmt.Println("Set with: nube memory me set \"...\"")
				return nil
			}
			fmt.Println(body.Content)
			return nil
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "set <content>",
		Short: "Replace your memory",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			content := strings.Join(args, " ")
			status, data, err := client.Do("PUT", "/api/memory/me", map[string]string{"content": content})
			if err != nil {
				return err
			}
			CheckError(status, data)
			fmt.Println("Your memory updated.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "add <content>",
		Short: "Append a line to your memory",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := NewClient()
			if err != nil {
				return err
			}
			content := strings.Join(args, " ")
			status, data, err := client.Do("POST", "/api/memory/me", map[string]string{"content": content})
			if err != nil {
				return err
			}
			CheckError(status, data)
			fmt.Println("Added to your memory.")
			return nil
		},
	})

	return cmd
}
