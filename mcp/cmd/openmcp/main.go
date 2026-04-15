package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	openapi2mcp "github.com/NubeIO/openapi-mcp"
	"github.com/NubeIO/openapi-mcp/pkg/config"
	"github.com/NubeIO/openapi-mcp/pkg/server"
	"github.com/spf13/cobra"
)

var cfg config.Config

func main() {
	root := &cobra.Command{
		Use:   "openmcp",
		Short: "Expose OpenAPI APIs as MCP tools",
	}

	// Persistent flags available to all subcommands.
	pf := root.PersistentFlags()
	pf.StringVar(&cfg.Spec, "spec", "", "path to OpenAPI spec (YAML/JSON)")
	pf.StringVar(&cfg.BaseURL, "base-url", "", "upstream API base URL")
	pf.StringVar(&cfg.Token, "token", "", "bearer token for upstream API")
	pf.StringVar(&cfg.APIKey, "api-key", "", "API key for upstream API")
	pf.StringVar(&cfg.APIKeyHeader, "api-key-header", "", "header name for API key")
	pf.StringVar(&cfg.Name, "name", "", "MCP server name")
	pf.StringVar(&cfg.Version, "version", "", "MCP server version")
	pf.StringSliceVar(&cfg.Tags, "tag", nil, "filter by OpenAPI tag (repeatable)")
	pf.BoolVar(&cfg.ReadOnly, "read-only", false, "only expose GET operations")
	pf.BoolVar(&cfg.ConfirmDangerous, "confirm-dangerous", true, "require confirmation for PUT/POST/DELETE")
	pf.BoolVar(&cfg.LogHTTP, "log-http", false, "log HTTP requests/responses")

	root.AddCommand(serveCmd(), lintCmd(), validateCmd(), toolsCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// serveCmd starts the MCP server.
func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server from an OpenAPI spec",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Allow spec as positional arg.
			if cfg.Spec == "" && len(args) > 0 {
				cfg.Spec = args[0]
			}
			transport, _ := cmd.Flags().GetString("transport")
			addr, _ := cmd.Flags().GetString("addr")
			plugins, _ := cmd.Flags().GetString("plugins")
			cfg.Transport = transport
			cfg.Addr = addr
			cfg.Plugins = plugins
			return server.QuickStart(cfg)
		},
	}
	cmd.Flags().String("transport", "stdio", "transport: stdio or http")
	cmd.Flags().String("addr", ":8080", "HTTP listen address")
	cmd.Flags().String("plugins", "", "plugins directory")
	return cmd
}

// lintCmd performs comprehensive linting on an OpenAPI spec.
func lintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [spec]",
		Short: "Lint an OpenAPI spec for MCP compatibility",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			specPath := args[0]
			doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
			if err != nil {
				return fmt.Errorf("loading spec: %w", err)
			}
			fmt.Fprintln(os.Stderr, "OpenAPI spec loaded successfully.")
			ops := openapi2mcp.ExtractOpenAPIOperations(doc)
			var toolNames []string
			for _, op := range ops {
				toolNames = append(toolNames, op.OperationID)
			}
			err = openapi2mcp.SelfTestOpenAPIMCPWithOptions(doc, toolNames, true)
			if err != nil {
				return fmt.Errorf("linting completed with issues: %w", err)
			}
			fmt.Fprintln(os.Stderr, "Linting passed: spec follows all best practices.")
			return nil
		},
	}
}

// validateCmd validates an OpenAPI spec and runs MCP self-test.
func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate [spec]",
		Short: "Validate an OpenAPI spec and run MCP self-test",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			specPath := args[0]
			doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
			fmt.Fprintln(os.Stderr, "OpenAPI spec loaded and validated successfully.")
			ops := openapi2mcp.ExtractOpenAPIOperations(doc)
			var toolNames []string
			for _, op := range ops {
				toolNames = append(toolNames, op.OperationID)
			}
			err = openapi2mcp.SelfTestOpenAPIMCPWithOptions(doc, toolNames, false)
			if err != nil {
				return fmt.Errorf("MCP self-test failed: %w", err)
			}
			fmt.Fprintln(os.Stderr, "MCP self-test passed: all tools and required arguments are present.")
			return nil
		},
	}
}

// toolsCmd lists the tools that would be generated from a spec.
func toolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools [spec]",
		Short: "List tools that would be generated from an OpenAPI spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specPath := args[0]
			doc, err := openapi2mcp.LoadOpenAPISpec(specPath)
			if err != nil {
				return fmt.Errorf("loading spec: %w", err)
			}

			ops := openapi2mcp.ExtractOpenAPIOperations(doc)

			// Apply tag filter.
			if len(cfg.Tags) > 0 {
				ops = filterByTags(ops, cfg.Tags)
			}

			// Apply read-only filter.
			if cfg.ReadOnly {
				ops = filterReadOnly(ops)
			}

			// Apply include/exclude regex from flags.
			includeRegex, _ := cmd.Flags().GetString("include-regex")
			excludeRegex, _ := cmd.Flags().GetString("exclude-regex")
			if includeRegex != "" || excludeRegex != "" {
				var incRe, excRe *regexp.Regexp
				if includeRegex != "" {
					incRe, err = regexp.Compile(includeRegex)
					if err != nil {
						return fmt.Errorf("invalid --include-regex: %w", err)
					}
				}
				if excludeRegex != "" {
					excRe, err = regexp.Compile(excludeRegex)
					if err != nil {
						return fmt.Errorf("invalid --exclude-regex: %w", err)
					}
				}
				ops = filterByRegex(ops, incRe, excRe)
			}

			for _, op := range ops {
				method := strings.ToUpper(op.Method)
				fmt.Printf("%-8s %-40s %s %s\n", method, op.OperationID, op.Path, joinTags(op.Tags))
			}
			fmt.Fprintf(os.Stderr, "\n%d tools\n", len(ops))
			return nil
		},
	}
	cmd.Flags().String("include-regex", "", "only include operations matching regex")
	cmd.Flags().String("exclude-regex", "", "exclude operations matching regex")
	return cmd
}

// --- helpers ---

func filterByTags(ops []openapi2mcp.OpenAPIOperation, tags []string) []openapi2mcp.OpenAPIOperation {
	tagSet := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}
	var filtered []openapi2mcp.OpenAPIOperation
	for _, op := range ops {
		for _, t := range op.Tags {
			if _, ok := tagSet[t]; ok {
				filtered = append(filtered, op)
				break
			}
		}
	}
	return filtered
}

func filterReadOnly(ops []openapi2mcp.OpenAPIOperation) []openapi2mcp.OpenAPIOperation {
	var filtered []openapi2mcp.OpenAPIOperation
	for _, op := range ops {
		if strings.EqualFold(op.Method, "get") {
			filtered = append(filtered, op)
		}
	}
	return filtered
}

func filterByRegex(ops []openapi2mcp.OpenAPIOperation, include, exclude *regexp.Regexp) []openapi2mcp.OpenAPIOperation {
	var filtered []openapi2mcp.OpenAPIOperation
	for _, op := range ops {
		desc := op.Description
		if desc == "" {
			desc = op.Summary
		}
		if include != nil && !include.MatchString(desc) {
			continue
		}
		if exclude != nil && exclude.MatchString(desc) {
			continue
		}
		filtered = append(filtered, op)
	}
	return filtered
}

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "[" + strings.Join(tags, ", ") + "]"
}
