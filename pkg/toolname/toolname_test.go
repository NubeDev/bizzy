package toolname

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		// Valid regular tools.
		{"query_nodes", "", false},
		{"get_weather", "", false},
		{"list_items", "", false},
		{"ab", "", false}, // minimum length

		// Valid QA tools.
		{"travel_quiz_qa", "qa", false},
		{"content_review_qa", "qa", false},

		// Valid prompt-mode tools (no suffix required).
		{"navigation", "prompt", false},
		{"build_dashboard", "prompt", false},

		// Invalid: empty.
		{"", "", true},

		// Invalid: starts with underscore.
		{"_helpers", "", true},

		// Invalid: uppercase.
		{"QueryNodes", "", true},

		// Invalid: hyphens (use underscores).
		{"query-nodes", "", true},

		// Invalid: too short.
		{"a", "", true},

		// Invalid: suffix/mode mismatch.
		{"travel_quiz_qa", "", false},  // suffix with no mode is OK (mode can be inferred)
		{"travel_quiz_qa", "prompt", true}, // _qa suffix but mode says "prompt"
		{"query_nodes", "qa", true},    // mode is "qa" but missing _qa suffix

		// Invalid: starts with digit.
		{"1tool", "", true},
	}

	for _, tt := range tests {
		err := Validate(tt.name, tt.mode)
		if (err != nil) != tt.wantErr {
			t.Errorf("Validate(%q, %q) = %v, wantErr=%v", tt.name, tt.mode, err, tt.wantErr)
		}
	}
}

func TestParse(t *testing.T) {
	info := Parse("content_review_qa")
	if info.Suffix != "_qa" {
		t.Errorf("expected suffix _qa, got %q", info.Suffix)
	}
	if info.ImpliedMode != "qa" {
		t.Errorf("expected mode qa, got %q", info.ImpliedMode)
	}
	if info.BaseName != "content_review" {
		t.Errorf("expected base content_review, got %q", info.BaseName)
	}

	info = Parse("query_nodes")
	if info.Suffix != "" {
		t.Errorf("expected no suffix, got %q", info.Suffix)
	}
	if info.BaseName != "query_nodes" {
		t.Errorf("expected base query_nodes, got %q", info.BaseName)
	}

	info = Parse("_helpers")
	if !info.IsHelper {
		t.Error("expected IsHelper=true")
	}
}

func TestSuggestName(t *testing.T) {
	tests := []struct {
		base, mode, want string
	}{
		{"content_review", "qa", "content_review_qa"},
		{"content_review_qa", "qa", "content_review_qa"}, // don't double-suffix
		{"navigation", "prompt", "navigation"},
		{"get_weather", "", "get_weather"},
	}

	for _, tt := range tests {
		got := SuggestName(tt.base, tt.mode)
		if got != tt.want {
			t.Errorf("SuggestName(%q, %q) = %q, want %q", tt.base, tt.mode, got, tt.want)
		}
	}
}

func TestInferMode(t *testing.T) {
	if got := InferMode("quiz_qa"); got != "qa" {
		t.Errorf("InferMode(quiz_qa) = %q, want qa", got)
	}
	if got := InferMode("get_weather"); got != "" {
		t.Errorf("InferMode(get_weather) = %q, want empty", got)
	}
}

func TestRegisterSuffix(t *testing.T) {
	RegisterSuffix("_wizard", "wizard")
	defer func() { delete(suffixes, "_wizard") }()

	if err := Validate("setup_wizard", "wizard"); err != nil {
		t.Errorf("expected valid after registering _wizard: %v", err)
	}
	if err := Validate("setup", "wizard"); err == nil {
		t.Error("expected error: mode wizard without _wizard suffix")
	}
	if got := InferMode("setup_wizard"); got != "wizard" {
		t.Errorf("InferMode(setup_wizard) = %q, want wizard", got)
	}
}
