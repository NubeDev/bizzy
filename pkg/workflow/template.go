package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var templateRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// Resolve replaces all {{variable}} placeholders in a string using the given
// variable map. Supports dotted paths like {{product_data.name}}.
func Resolve(tmpl string, vars map[string]any) string {
	return templateRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		// Extract the key between {{ and }}.
		key := strings.TrimSpace(match[2 : len(match)-2])
		val, ok := lookupPath(vars, key)
		if !ok {
			return match // leave unresolved
		}
		return toString(val)
	})
}

// ResolveParams resolves all template variables in a params map.
func ResolveParams(params map[string]string, vars map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for k, v := range params {
		out[k] = Resolve(v, vars)
	}
	return out
}

// lookupPath resolves a dotted path like "inputs.product" or "product_data.specs"
// against a nested map.
func lookupPath(vars map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = vars

	for _, part := range parts {
		switch m := current.(type) {
		case map[string]any:
			val, ok := m[part]
			if !ok {
				return nil, false
			}
			current = val
		case map[string]string:
			val, ok := m[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}
	return current, true
}

// toString converts a value to its string representation.
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		// For complex types (maps, slices), marshal to JSON.
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}
