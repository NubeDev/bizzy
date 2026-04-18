// Package version provides a single source of truth for all versioned
// contracts in bizzy. Any package can import this without creating
// circular dependencies — it has zero bizzy imports.
//
// Bump a version when the contract changes in a way that existing
// consumers need to know about. Use semver:
//   - MAJOR: breaking change (old clients will fail)
//   - MINOR: new capability (old clients still work)
//   - PATCH: bug fix in the contract implementation
package version

// Server is the overall server version.
const Server = "0.1.0"

// CommandSyntax is the command parsing contract version.
// Bump when: verbs change, target resolution rules change,
// shorthand rules change, or ParseConfig semantics change.
const CommandSyntax = "1.0.0"

// BusTopics is the event bus topic taxonomy version.
// Bump when: topic names change, EventData fields change,
// new required fields are added, or topic hierarchy is restructured.
const BusTopics = "1.0.0"

// API is the REST API contract version.
// Bump when: endpoints change paths, request/response shapes change,
// or auth requirements change.
const API = "1.0.0"

// AdapterProtocol is the adapter interface version.
// Bump when: the Adapter interface changes, ReplyInfo format changes,
// or typed address struct fields change.
const AdapterProtocol = "1.0.0"

// NotifyPrefs is the notification preferences schema version.
// Bump when: preference fields are added/removed or fan-out
// behaviour changes.
const NotifyPrefs = "1.0.0"

// PluginProtocol is the plugin registration and communication protocol version.
// Bump when: manifest schema changes, NATS message formats change,
// tool call request/response shapes change, or health protocol changes.
const PluginProtocol = "1.0.0"

// All returns every version as a map, useful for health endpoints
// and diagnostics.
func All() map[string]string {
	return map[string]string{
		"server":           Server,
		"command_syntax":   CommandSyntax,
		"bus_topics":       BusTopics,
		"api":              API,
		"adapter_protocol": AdapterProtocol,
		"notify_prefs":     NotifyPrefs,
		"plugin_protocol":  PluginProtocol,
	}
}
