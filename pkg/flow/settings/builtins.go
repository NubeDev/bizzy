package settings

// onErrorProp returns the common on_error property used by most nodes.
func onErrorProp() *JSONSchema {
	return String().
		Title("On Error").
		Desc("What to do when this node fails.").
		Default("stop").
		Enum("stop", "skip", "retry", "fallback").
		Build()
}

// maxRetriesProp returns the max_retries property (shown when on_error=retry).
func maxRetriesProp() *JSONSchema {
	return Integer().
		Title("Max Retries").
		Desc("Number of retry attempts before falling through to stop.").
		Default(3).
		Range(1, 10).
		Build()
}

// timeoutProp returns the per-node timeout property.
func timeoutProp() *JSONSchema {
	return Integer().
		Title("Timeout (seconds)").
		Desc("Max execution time for this node. 0 = no limit.").
		Default(0).
		Min(0).
		Build()
}

// --- Flow control node schemas ---

func TriggerSchema() *JSONSchema {
	return Object().
		Title("Trigger Settings").
		Property("type", String().
			Title("Trigger Type").
			Desc("How this flow is started.").
			Default("manual").
			Enum("manual", "cron", "interval", "webhook", "event").
			Build()).
		Property("schedule", String().
			Title("Schedule").
			Desc("Cron expression (e.g. '0 9 * * 1-5') for cron type, or duration (e.g. '10s', '5m', '1h') for interval type.").
			Build()).
		Property("webhook_path", String().
			Title("Webhook Path").
			Desc("URL path suffix for incoming webhooks. Only used when type is 'webhook'.").
			Build()).
		Property("event", String().
			Title("Event Topic").
			Desc("NATS topic pattern to subscribe to (e.g. 'sensor.>'). Only used when type is 'event'.").
			Build()).
		Property("filter", Object().
			Title("Event Filter").
			Desc("Key-value filter applied to incoming events. Only matching events trigger the flow.").
			Build()).
		Build()
}

func ApprovalSchema() *JSONSchema {
	return Object().
		Title("Approval Settings").
		Property("message", String().
			Title("Approval Message").
			Desc("Message shown to the approver.").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func ConditionSchema() *JSONSchema {
	return Object().
		Title("Condition Settings").
		Property("expression", String().
			Title("Expression").
			Desc("Expression that returns true or false. Access input data via 'input'.").
			Widget("code").
			MinLen(1).
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Required("expression").
		Build()
}

func SwitchSchema() *JSONSchema {
	return Object().
		Title("Switch Settings").
		Property("expression", String().
			Title("Expression").
			Desc("Expression whose result determines the output port (case_<value> or default).").
			Widget("code").
			MinLen(1).
			Build()).
		Property("cases", Object().
			Title("Cases").
			Desc("Map of case values. Each key becomes a 'case_<key>' output port.").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Required("expression").
		Build()
}

func MergeSchema() *JSONSchema {
	return Object().
		Title("Merge Settings").
		Property("timeout", timeoutProp()).
		Build()
}

func RaceSchema() *JSONSchema {
	return Object().
		Title("Race Settings").
		Property("timeout", timeoutProp()).
		Build()
}

func ForEachSchema() *JSONSchema {
	return Object().
		Title("ForEach Settings").
		Property("max_iterations", Integer().
			Title("Max Iterations").
			Desc("Maximum number of items to process. Hard cap: 10000.").
			Default(1000).
			Range(1, 10000).
			Build()).
		Property("concurrency", Integer().
			Title("Concurrency").
			Desc("Number of items processed in parallel.").
			Default(10).
			Range(1, 100).
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func DelaySchema() *JSONSchema {
	return Object().
		Title("Delay Settings").
		Property("duration", String().
			Title("Duration").
			Desc("How long to wait. Examples: 1s, 500ms, 2m, 1h.").
			Default("1s").
			Pattern(`^\d+(\.\d+)?(ms|s|m|h)$`).
			Build()).
		Property("timeout", timeoutProp()).
		Required("duration").
		Build()
}

func OutputSchema() *JSONSchema {
	return Object().
		Title("Output Settings").
		Build()
}

func ErrorSchema() *JSONSchema {
	return Object().
		Title("Error Settings").
		Build()
}

// --- Data node schemas ---

func ValueSchema() *JSONSchema {
	return Object().
		Title("Value Settings").
		Property("value", String().
			Title("Value").
			Desc("Static JSON value to emit. Enter raw JSON: string, number, object, or array.").
			Widget("json").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func TemplateSchema() *JSONSchema {
	return Object().
		Title("Template Settings").
		Property("template", String().
			Title("Template").
			Desc("Go template string. Use {{.input}}, {{.varName}} placeholders.").
			Widget("code").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func HTTPRequestSchema() *JSONSchema {
	return Object().
		Title("HTTP Request Settings").
		Property("url", String().
			Title("URL").
			URL().
			Default("https://").
			Build()).
		Property("method", String().
			Title("Method").
			Default("GET").
			Enum("GET", "POST", "PUT", "PATCH", "DELETE").
			Build()).
		Property("headers", Object().
			Title("Headers").
			Desc("Key-value pairs sent as HTTP headers.").
			Build()).
		Property("timeout", Integer().
			Title("Timeout (seconds)").
			Default(30).
			Range(1, 300).
			Build()).
		Property("on_error", onErrorProp()).
		Required("url").
		Build()
}

func TransformSchema() *JSONSchema {
	return Object().
		Title("Transform Settings").
		Property("expression", String().
			Title("Expression").
			Desc("Expression to reshape data. Access input data via 'input', flow variables via 'vars'.").
			Widget("code").
			MinLen(1).
			Build()).
		Property("on_error", onErrorProp()).
		Property("max_retries", maxRetriesProp()).
		Property("timeout", timeoutProp()).
		Required("expression").
		Build()
}

func SetVariableSchema() *JSONSchema {
	return Object().
		Title("Set Variable Settings").
		Property("variable", String().
			Title("Variable Name").
			Desc("Name of the flow variable to set.").
			MinLen(1).
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Required("variable").
		Build()
}

func LogSchema() *JSONSchema {
	return Object().
		Title("Log Settings").
		Property("message", String().
			Title("Message").
			Desc("Custom log message. Leave empty to log the input value.").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func CounterSchema() *JSONSchema {
	return Object().
		Title("Counter Settings").
		Property("variable", String().
			Title("Variable Name").
			Desc("Name of the flow variable to store the count in.").
			Default("counter").
			Build()).
		Property("operation", String().
			Title("Operation").
			Desc("What to do with the counter.").
			Default("increment").
			Enum("increment", "decrement", "reset", "set").
			Build()).
		Property("step", Integer().
			Title("Step").
			Desc("Amount to increment or decrement by.").
			Default(1).
			Min(1).
			Build()).
		Property("initial", Integer().
			Title("Initial Value").
			Desc("Starting value when the counter variable doesn't exist yet.").
			Default(0).
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

// --- Integration node schemas ---

func AIPromptSchema() *JSONSchema {
	return Object().
		Title("AI Prompt Settings").
		Property("prompt", String().
			Title("Prompt").
			Desc("The prompt text. Can also be provided via the input port.").
			Widget("textarea").
			Build()).
		Property("provider", String().
			Title("Provider").
			Desc("AI provider to use. Leave empty for default.").
			Enum("claude", "openai", "ollama").
			Build()).
		Property("model", String().
			Title("Model").
			Desc("Model override. Leave empty for provider default.").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func AIRunnerSchema() *JSONSchema {
	return Object().
		Title("AI Runner Settings").
		Property("provider", String().
			Title("Provider").
			Default("claude").
			Enum("claude", "opencode", "codex", "copilot", "ollama").
			Build()).
		Property("model", String().
			Title("Model").
			Desc("Model override. Leave empty for provider default.").
			Build()).
		Property("work_dir", String().
			Title("Working Directory").
			Desc("Repo directory for the AI session.").
			Build()).
		Property("thinking_budget", String().
			Title("Thinking Budget").
			Default("medium").
			Enum("low", "medium", "high").
			Build()).
		Property("allowed_tools", String().
			Title("Allowed Tools").
			Desc("MCP tool filter pattern.").
			Default("*").
			Build()).
		Property("timeout_mins", Integer().
			Title("Timeout (minutes)").
			Default(30).
			Range(1, 120).
			Build()).
		Property("resume_session", Bool().
			Title("Resume Session").
			Desc("Resume previous AI session for this node if one exists.").
			Default(false).
			Build()).
		Property("on_error", onErrorProp()).
		Required("provider").
		Build()
}

func SlackSendSchema() *JSONSchema {
	return Object().
		Title("Slack Send Settings").
		Property("channel", String().
			Title("Channel").
			Desc("Slack channel ID or name. Can also be provided via input port.").
			Build()).
		Property("message", String().
			Title("Message").
			Desc("Message text. Can also be provided via input port.").
			Widget("textarea").
			Build()).
		Property("thread_ts", String().
			Title("Thread TS").
			Desc("Reply to a specific thread.").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func EmailSendSchema() *JSONSchema {
	return Object().
		Title("Email Send Settings").
		Property("to", String().
			Title("To").
			Desc("Recipient email. Can also be provided via input port.").
			Email().
			Build()).
		Property("subject", String().
			Title("Subject").
			Desc("Email subject. Can also be provided via input port.").
			Build()).
		Property("body", String().
			Title("Body").
			Desc("Email body. Can also be provided via input port.").
			Widget("textarea").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Build()
}

func WebhookCallSchema() *JSONSchema {
	return Object().
		Title("Webhook Call Settings").
		Property("url", String().
			Title("URL").
			URL().
			Default("https://").
			Build()).
		Property("method", String().
			Title("Method").
			Default("GET").
			Enum("GET", "POST", "PUT", "PATCH", "DELETE").
			Build()).
		Property("on_error", onErrorProp()).
		Property("timeout", timeoutProp()).
		Required("url").
		Build()
}

// BuiltinSchemas returns a map of node type -> settings schema for all built-in types.
func BuiltinSchemas() map[string]*JSONSchema {
	return map[string]*JSONSchema{
		// Flow control.
		"trigger":  TriggerSchema(),
		"approval": ApprovalSchema(),
		"condition": ConditionSchema(),
		"switch":    SwitchSchema(),
		"merge":     MergeSchema(),
		"race":      RaceSchema(),
		"foreach":   ForEachSchema(),
		"delay":     DelaySchema(),
		"output":    OutputSchema(),
		"error":     ErrorSchema(),
		// Data.
		"value":        ValueSchema(),
		"template":     TemplateSchema(),
		"http-request": HTTPRequestSchema(),
		"transform":    TransformSchema(),
		"set-variable": SetVariableSchema(),
		"log":          LogSchema(),
		"counter":      CounterSchema(),
		// Integration.
		"ai-prompt":    AIPromptSchema(),
		"ai-runner":    AIRunnerSchema(),
		"slack-send":   SlackSendSchema(),
		"email-send":   EmailSendSchema(),
		"webhook-call": WebhookCallSchema(),
	}
}
