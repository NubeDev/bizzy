// Package toolname validates and classifies tool names based on naming conventions.
//
// Tool files follow a suffix-based convention:
//
//	tool_name.js + .json      → regular tool (mode "")
//	tool_name_qa.js + .json   → QA/interactive tool (mode "qa")
//	tool_name.json (no .js)   → prompt-mode tool (mode "prompt")
//	_helpers.js               → shared helper, not a tool
//
// New suffixes can be added by registering them with RegisterSuffix.
package toolname

import (
	"fmt"
	"regexp"
	"strings"
)

// validBase matches the base name: lowercase, underscores, digits.
var validBase = regexp.MustCompile(`^[a-z][a-z0-9_]{0,58}[a-z0-9]$`)

// suffix maps a known suffix to the mode it implies.
// The empty string key is the default (no suffix → regular tool).
var suffixes = map[string]string{
	"_qa": "qa",
}

// RegisterSuffix adds a new suffix→mode mapping. Call at init time to extend
// the convention for future tool types (e.g. "_wizard" → "wizard").
func RegisterSuffix(suffix, mode string) {
	if suffix == "" || mode == "" {
		return
	}
	suffixes[suffix] = mode
}

// Suffixes returns a copy of the registered suffix→mode mappings.
func Suffixes() map[string]string {
	out := make(map[string]string, len(suffixes))
	for k, v := range suffixes {
		out[k] = v
	}
	return out
}

// Info holds the parsed result of a tool name.
type Info struct {
	Name         string // full name as provided (e.g. "travel_quiz_qa")
	BaseName     string // name without suffix (e.g. "travel_quiz")
	Suffix       string // matched suffix (e.g. "_qa") or ""
	ImpliedMode  string // mode the suffix implies (e.g. "qa") or ""
	IsHelper     bool   // true if name starts with "_"
}

// Parse extracts naming info from a tool name without validating.
func Parse(name string) Info {
	info := Info{Name: name}

	if strings.HasPrefix(name, "_") {
		info.IsHelper = true
		return info
	}

	// Check registered suffixes (longest first to avoid partial matches).
	for suffix, mode := range suffixes {
		if strings.HasSuffix(name, suffix) {
			info.Suffix = suffix
			info.ImpliedMode = mode
			info.BaseName = strings.TrimSuffix(name, suffix)
			return info
		}
	}

	info.BaseName = name
	return info
}

// Validate checks that a tool name is well-formed and that the name suffix
// matches the declared mode. Returns nil if valid.
//
// Rules:
//   - Must match ^[a-z][a-z0-9_]{0,58}[a-z0-9]$ (lowercase, underscores, 2-60 chars)
//   - Must not start with "_" (reserved for helpers)
//   - If name ends with a known suffix (e.g. "_qa"), mode must match
//   - If mode is set but name has no suffix, the correct suffix is suggested
func Validate(name, mode string) error {
	if name == "" {
		return fmt.Errorf("tool name is required")
	}

	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("tool name must not start with _ (reserved for helpers like _helpers.js)")
	}

	if !validBase.MatchString(name) {
		return fmt.Errorf("tool name %q is invalid: must be 2-60 chars, lowercase letters, digits, and underscores", name)
	}

	info := Parse(name)

	// Name has a known suffix — mode must match.
	if info.Suffix != "" && mode != "" && mode != info.ImpliedMode {
		return fmt.Errorf("tool name %q has suffix %q which implies mode %q, but mode is %q", name, info.Suffix, info.ImpliedMode, mode)
	}

	// Mode is set but name doesn't have the expected suffix.
	if mode != "" && mode != "prompt" && info.Suffix == "" {
		expectedSuffix := suffixForMode(mode)
		if expectedSuffix != "" {
			return fmt.Errorf("tool with mode %q must have suffix %q (e.g. %q)", mode, expectedSuffix, name+expectedSuffix)
		}
	}

	return nil
}

// InferMode returns the mode implied by the tool name's suffix.
// Returns empty string for regular tools or if the name has no known suffix.
func InferMode(name string) string {
	return Parse(name).ImpliedMode
}

// SuggestName returns the correctly-suffixed name for a given base name and mode.
// If mode is empty or "prompt", returns the base name unchanged.
func SuggestName(baseName, mode string) string {
	if mode == "" || mode == "prompt" {
		return baseName
	}
	suffix := suffixForMode(mode)
	if suffix == "" {
		return baseName
	}
	// Don't double-suffix.
	if strings.HasSuffix(baseName, suffix) {
		return baseName
	}
	return baseName + suffix
}

func suffixForMode(mode string) string {
	for suffix, m := range suffixes {
		if m == mode {
			return suffix
		}
	}
	return ""
}
