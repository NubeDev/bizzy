package settings

// MultipleSettingsSchemas supports nodes that have distinct configuration
// modes, each with its own schema. For example, ai-runner can target
// different providers, each requiring different fields.
type MultipleSettingsSchemas struct {
	SupportsMultiple bool                 `json:"supportsMultiple"`
	Schemas          []NamedSettingsSchema `json:"schemas"`
	DefaultSchema    string               `json:"defaultSchema"`
}

// NamedSettingsSchema pairs a display name with a JSON Schema.
type NamedSettingsSchema struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"displayName"`
	Description string     `json:"description,omitempty"`
	Schema      JSONSchema `json:"schema"`
}
