package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewLoginCmd creates the "nube login" command.
func NewLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login <server-url> <token>",
		Short: "Save server URL and token to ~/.nube/config.json",
		Long: `Authenticate with a nube-server instance.

Examples:
  nube login http://localhost:8090 abc123def456
  nube login https://nube.example.com my-token`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			server := args[0]
			token := args[1]

			// Verify the token works.
			client := NewClientFrom(server, token)
			status, data, err := client.Do("GET", "/users/me", nil)
			if err != nil {
				return fmt.Errorf("cannot reach server: %w", err)
			}
			if status != 200 {
				CheckError(status, data)
			}

			cfg := &Config{Server: server, Token: token}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("Logged in to %s\n", server)
			fmt.Printf("Config saved to %s\n", ConfigPath())
			return nil
		},
	}
}

// NewLogoutCmd creates the "nube logout" command.
func NewLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear saved credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := &Config{}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Println("Logged out.")
			return nil
		},
	}
}
