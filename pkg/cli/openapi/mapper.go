package openapi

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/NubeDev/bizzy/pkg/cli"
	"github.com/spf13/cobra"
)

// RegisterCommands reads the OpenAPI spec and registers cobra commands on the root.
func RegisterCommands(root *cobra.Command, specData []byte) error {
	ops, err := ParseSpec(specData)
	if err != nil {
		return err
	}

	// Group commands by their first segment (e.g. "apps", "users").
	groups := make(map[string]*cobra.Command)

	for _, op := range ops {
		parts := strings.Fields(op.Command)

		var parent *cobra.Command
		if len(parts) == 1 {
			// Top-level command: "status", "bootstrap"
			parent = root
		} else {
			// Group command: "apps list" → parent is "apps"
			groupName := parts[0]
			if _, ok := groups[groupName]; !ok {
				groups[groupName] = &cobra.Command{
					Use:   groupName,
					Short: fmt.Sprintf("Manage %s", groupName),
				}
				root.AddCommand(groups[groupName])
			}
			parent = groups[groupName]
			parts = parts[1:] // remaining is the subcommand
		}

		cmd := buildCommand(op, parts)
		parent.AddCommand(cmd)
	}

	return nil
}

func buildCommand(op CLIOp, nameParts []string) *cobra.Command {
	name := strings.Join(nameParts, "-")

	// Build usage string with positional args.
	usage := name
	for _, p := range op.PathParams {
		usage += " <" + p.Name + ">"
	}

	cmd := &cobra.Command{
		Use:   usage,
		Short: op.Summary,
		RunE:  makeHandler(op),
	}

	// Register flags for query params.
	for _, p := range op.QueryParams {
		cmd.Flags().String(p.Name, "", p.Description)
	}

	// Register flags for body fields.
	for _, f := range op.BodyFields {
		desc := f.Desc
		if len(f.Enum) > 0 {
			desc += fmt.Sprintf(" [%s]", strings.Join(f.Enum, "|"))
		}
		if f.Default != nil {
			desc += fmt.Sprintf(" (default: %v)", f.Default)
		}

		switch f.Type {
		case "boolean":
			cmd.Flags().Bool(f.Name, false, desc)
		case "object":
			// Object fields (like settings) are passed as key=value pairs.
			cmd.Flags().StringSlice(f.Name, nil, desc+" (key=value pairs)")
		default:
			cmd.Flags().String(f.Name, "", desc)
		}
	}

	return cmd
}

func makeHandler(op CLIOp) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Get client.
		var client *cli.Client
		var err error
		if op.NoAuth {
			client, err = cli.GetUnauthClient()
		} else {
			client, err = cli.GetClient()
		}
		if err != nil {
			return err
		}

		// Build path with positional args.
		path := op.Path
		for i, p := range op.PathParams {
			if i >= len(args) {
				return fmt.Errorf("missing required argument: %s", p.Name)
			}
			path = strings.Replace(path, "{"+p.Name+"}", args[i], 1)
		}

		// Add query params from flags.
		if len(op.QueryParams) > 0 {
			q := url.Values{}
			for _, p := range op.QueryParams {
				val, _ := cmd.Flags().GetString(p.Name)
				if val != "" {
					q.Set(p.Name, val)
				}
			}
			if encoded := q.Encode(); encoded != "" {
				path += "?" + encoded
			}
		}

		// Build body from flags.
		var body any
		if len(op.BodyFields) > 0 {
			bodyMap := make(map[string]any)
			hasBody := false

			for _, f := range op.BodyFields {
				switch f.Type {
				case "boolean":
					if cmd.Flags().Changed(f.Name) {
						val, _ := cmd.Flags().GetBool(f.Name)
						bodyMap[f.Name] = val
						hasBody = true
					}
				case "object":
					// Parse key=value pairs into a map.
					vals, _ := cmd.Flags().GetStringSlice(f.Name)
					if len(vals) > 0 {
						m := make(map[string]string)
						for _, kv := range vals {
							parts := strings.SplitN(kv, "=", 2)
							if len(parts) == 2 {
								m[parts[0]] = parts[1]
							}
						}
						bodyMap[f.Name] = m
						hasBody = true
					}
				default:
					val, _ := cmd.Flags().GetString(f.Name)
					if val != "" {
						bodyMap[f.Name] = val
						hasBody = true
					}
				}
			}

			if hasBody {
				body = bodyMap
			}
		}

		// For POST/PUT/PATCH with no flags set, send empty body.
		if body == nil && (op.Method == "POST" || op.Method == "PUT" || op.Method == "PATCH") {
			body = map[string]any{}
		}

		// Execute request.
		status, data, err := client.Do(op.Method, path, body)
		if err != nil {
			return err
		}

		cli.CheckError(status, data)

		// Output.
		if cli.IsJSON() {
			cli.PrintRawJSON(data)
			return nil
		}

		// Try to auto-format.
		var arr []map[string]any
		if json.Unmarshal(data, &arr) == nil {
			columns := guessColumns(op)
			cli.PrintTable(arr, columns)
			return nil
		}

		var obj map[string]any
		if json.Unmarshal(data, &obj) == nil {
			// If it has a nested object (like app detail), show as JSON.
			for _, v := range obj {
				if _, isMap := v.(map[string]any); isMap {
					cli.PrintRawJSON(data)
					return nil
				}
				if _, isArr := v.([]any); isArr {
					cli.PrintRawJSON(data)
					return nil
				}
			}
			keys := guessObjectKeys(op)
			cli.PrintObject(obj, keys)
			return nil
		}

		// Fallback: raw output.
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}
}

// guessColumns returns table columns based on the command context.
func guessColumns(op CLIOp) []string {
	switch {
	case strings.Contains(op.Command, "tools"):
		return []string{"name", "appName", "type", "description"}
	case strings.Contains(op.Command, "prompts"):
		return []string{"name", "appName", "description"}
	case strings.Contains(op.Command, "apps"):
		return []string{"name", "version", "description", "hasOpenAPI", "hasTools", "hasPrompts"}
	case strings.Contains(op.Command, "installs"):
		return []string{"id", "appName", "appVersion", "enabled", "stale"}
	case strings.Contains(op.Command, "users"):
		return []string{"id", "name", "email", "role", "workspaceId"}
	case strings.Contains(op.Command, "workspaces"):
		return []string{"id", "name", "createdAt"}
	default:
		return nil // auto-detect
	}
}

func guessObjectKeys(op CLIOp) []string {
	switch {
	case strings.Contains(op.Command, "status"):
		return []string{"status", "users", "apps"}
	case strings.Contains(op.Command, "users"):
		return []string{"id", "name", "email", "role", "workspaceId", "token"}
	default:
		return nil
	}
}
