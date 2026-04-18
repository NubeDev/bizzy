// Package settings provides a JSON Schema builder for flow node configuration.
// Schemas define the settings forms that appear when a user selects a node on
// the canvas. The frontend renders them with @json-render/shadcn.
package settings

// JSONSchema is a JSON Schema object that can describe node settings.
// It supports enough of the spec for form generation: types, properties,
// required fields, validation constraints, enums, conditional logic, and
// UI hints for the renderer.
type JSONSchema struct {
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Default     any                    `json:"default,omitempty"`
	Enum        []any                  `json:"enum,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"`
	ReadOnly    bool                   `json:"readOnly,omitempty"`
	UIWidget    string                 `json:"ui:widget,omitempty"`
	UIHidden    bool                   `json:"ui:hidden,omitempty"`

	// Conditional (if/then/else).
	If   *JSONSchema `json:"if,omitempty"`
	Then *JSONSchema `json:"then,omitempty"`
	Else *JSONSchema `json:"else,omitempty"`
}

// --- Fluent builder ---

// Builder constructs a JSONSchema via chained method calls.
type Builder struct {
	schema JSONSchema
}

// String starts building a string schema.
func String() *Builder {
	return &Builder{schema: JSONSchema{Type: "string"}}
}

// Integer starts building an integer schema.
func Integer() *Builder {
	return &Builder{schema: JSONSchema{Type: "integer"}}
}

// Number starts building a number schema.
func Number() *Builder {
	return &Builder{schema: JSONSchema{Type: "number"}}
}

// Bool starts building a boolean schema.
func Bool() *Builder {
	return &Builder{schema: JSONSchema{Type: "boolean"}}
}

// Object starts building an object schema.
func Object() *Builder {
	return &Builder{schema: JSONSchema{Type: "object", Properties: make(map[string]*JSONSchema)}}
}

// Array starts building an array schema.
func Array() *Builder {
	return &Builder{schema: JSONSchema{Type: "array"}}
}

// Build returns the finished schema.
func (b *Builder) Build() *JSONSchema {
	s := b.schema
	return &s
}

// --- Common setters ---

func (b *Builder) Title(t string) *Builder {
	b.schema.Title = t
	return b
}

func (b *Builder) Desc(d string) *Builder {
	b.schema.Description = d
	return b
}

func (b *Builder) Default(v any) *Builder {
	b.schema.Default = v
	return b
}

func (b *Builder) Enum(values ...any) *Builder {
	b.schema.Enum = values
	return b
}

func (b *Builder) ReadOnly(v bool) *Builder {
	b.schema.ReadOnly = v
	return b
}

// --- String constraints ---

func (b *Builder) MinLen(n int) *Builder {
	b.schema.MinLength = &n
	return b
}

func (b *Builder) MaxLen(n int) *Builder {
	b.schema.MaxLength = &n
	return b
}

func (b *Builder) Pattern(p string) *Builder {
	b.schema.Pattern = p
	return b
}

func (b *Builder) Format(f string) *Builder {
	b.schema.Format = f
	return b
}

// URL is a shorthand for Format("uri").
func (b *Builder) URL() *Builder {
	return b.Format("uri")
}

// Email is a shorthand for Format("email").
func (b *Builder) Email() *Builder {
	return b.Format("email")
}

// --- Numeric constraints ---

func (b *Builder) Min(n float64) *Builder {
	b.schema.Minimum = &n
	return b
}

func (b *Builder) Max(n float64) *Builder {
	b.schema.Maximum = &n
	return b
}

// Range sets both minimum and maximum.
func (b *Builder) Range(min, max float64) *Builder {
	b.schema.Minimum = &min
	b.schema.Maximum = &max
	return b
}

// --- Object methods ---

// Property adds a named property to an object schema.
func (b *Builder) Property(name string, prop *JSONSchema) *Builder {
	if b.schema.Properties == nil {
		b.schema.Properties = make(map[string]*JSONSchema)
	}
	b.schema.Properties[name] = prop
	return b
}

// Required marks property names as required.
func (b *Builder) Required(names ...string) *Builder {
	b.schema.Required = append(b.schema.Required, names...)
	return b
}

// --- Array methods ---

// ItemsOf sets the schema for array items.
func (b *Builder) ItemsOf(item *JSONSchema) *Builder {
	b.schema.Items = item
	return b
}

// --- UI hints ---

// Widget sets the ui:widget hint for the frontend renderer.
func (b *Builder) Widget(w string) *Builder {
	b.schema.UIWidget = w
	return b
}

// Hidden marks the field as hidden in the form.
func (b *Builder) Hidden() *Builder {
	b.schema.UIHidden = true
	return b
}

// --- Conditional ---

func (b *Builder) If(cond *JSONSchema) *Builder {
	b.schema.If = cond
	return b
}

func (b *Builder) Then(then *JSONSchema) *Builder {
	b.schema.Then = then
	return b
}

func (b *Builder) Else(els *JSONSchema) *Builder {
	b.schema.Else = els
	return b
}
