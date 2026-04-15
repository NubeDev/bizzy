package cli

import (
	"github.com/spf13/cobra"
)

var (
	flagServer string
	flagToken  string
	flagOutput string
)

// NewRootCmd creates the root cobra command with global flags.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "nube",
		Short: "NubeIO developer tools CLI",
		Long:  "CLI for the NubeIO central server — manage workspaces, users, apps, and tools.",
	}

	root.PersistentFlags().StringVar(&flagServer, "server", "", "Server URL (overrides config)")
	root.PersistentFlags().StringVar(&flagToken, "token", "", "Bearer token (overrides config)")
	root.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "Output format: table or json")

	return root
}

// GetClient returns an HTTP client using flag overrides or saved config.
func GetClient() (*Client, error) {
	if flagServer != "" && flagToken != "" {
		return NewClientFrom(flagServer, flagToken), nil
	}

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// Flags override config.
	server := cfg.Server
	token := cfg.Token
	if flagServer != "" {
		server = flagServer
	}
	if flagToken != "" {
		token = flagToken
	}

	if server == "" {
		return nil, ErrNotLoggedIn
	}

	return NewClientFrom(server, token), nil
}

// GetUnauthClient returns a client that may not have a token (for bootstrap/health).
func GetUnauthClient() (*Client, error) {
	if flagServer != "" {
		return NewClientFrom(flagServer, flagToken), nil
	}
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Server == "" {
		return nil, ErrNotLoggedIn
	}
	return NewClientFrom(cfg.Server, cfg.Token), nil
}

// IsJSON returns true if --output=json.
func IsJSON() bool {
	return flagOutput == "json"
}

// ErrNotLoggedIn is returned when no server is configured.
var ErrNotLoggedIn = &LoginError{}

type LoginError struct{}

func (e *LoginError) Error() string {
	return "not logged in — run: nube login <server-url> <token>"
}
