package command

import (
	"fmt"
	"strings"
)

// ParseConfig controls how bare text is handled per adapter.
type ParseConfig struct {
	BareTextBehaviour string // "ask" | "require_mention" | "ignore" | "reject"
	MentionPrefix     string // "@bizzy" — stripped before parsing
}

// Parser converts raw text into Command structs.
type Parser struct {
	knownVerbs map[string]Verb
}

// NewParser creates a command parser.
func NewParser() *Parser {
	return &Parser{
		knownVerbs: map[string]Verb{
			"run":     VerbRun,
			"ask":     VerbAsk,
			"status":  VerbStatus,
			"cancel":  VerbCancel,
			"restart": VerbRestart,
			"list":    VerbList,
			"approve": VerbApprove,
			"reject":  VerbReject,
			"help":    VerbHelp,
		},
	}
}

// Parse converts raw text into a Command.
func (p *Parser) Parse(text string, userID string, replyTo ReplyInfo, cfg ParseConfig) (Command, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Command{}, fmt.Errorf("empty command")
	}

	// Strip mention prefix if present.
	if cfg.MentionPrefix != "" {
		text = strings.TrimPrefix(text, cfg.MentionPrefix)
		text = strings.TrimSpace(text)
	}

	cmd := NewCommand()
	cmd.UserID = userID
	cmd.ReplyTo = replyTo

	// Split into tokens.
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return Command{}, fmt.Errorf("empty command after parsing")
	}

	// Check if first token is a known verb.
	first := strings.ToLower(tokens[0])

	if verb, ok := p.knownVerbs[first]; ok {
		cmd.Verb = verb
		tokens = tokens[1:]
	} else if strings.HasPrefix(first, "/") {
		// Slash-command shorthand: /weekly-report → run workflow/weekly-report
		cmd.Verb = VerbRun
		name := strings.TrimPrefix(first, "/")
		cmd.Target = Target{Kind: "workflow", Name: name}
		tokens = tokens[1:]
		cmd.Params = parseParams(tokens)
		return cmd, nil
	} else {
		// No verb recognized — apply bare text behaviour.
		switch cfg.BareTextBehaviour {
		case "ask":
			cmd.Verb = VerbAsk
			cmd.Target = Target{Kind: "ai"}
			cmd.Params["prompt"] = text
			return cmd, nil
		case "ignore":
			return Command{}, fmt.Errorf("unrecognized command (ignored)")
		case "reject":
			return Command{}, fmt.Errorf("unrecognized command: %q — use 'help' for available commands", first)
		default: // "require_mention" — if we got here, mention was stripped, treat as ask
			cmd.Verb = VerbAsk
			cmd.Target = Target{Kind: "ai"}
			cmd.Params["prompt"] = text
			return cmd, nil
		}
	}

	// Special handling for certain verbs.
	switch cmd.Verb {
	case VerbHelp:
		return cmd, nil

	case VerbAsk:
		// Everything after "ask" is the prompt.
		prompt := strings.Join(tokens, " ")
		prompt = strings.Trim(prompt, "\"'")
		if prompt == "" {
			return Command{}, fmt.Errorf("ask requires a prompt")
		}
		cmd.Target = Target{Kind: "ai"}
		cmd.Params["prompt"] = prompt
		return cmd, nil

	case VerbApprove, VerbReject:
		// Target is optional (may be resolved from thread context).
		if len(tokens) > 0 {
			cmd.Target = parseTarget(tokens[0])
			tokens = tokens[1:]
		}
		if cmd.Verb == VerbReject {
			// Remaining tokens are feedback.
			feedback := parseParams(tokens)
			if f, ok := feedback["feedback"]; ok {
				cmd.Params["feedback"] = f
			} else if len(tokens) > 0 {
				cmd.Params["feedback"] = strings.Join(tokens, " ")
			}
		}
		return cmd, nil

	case VerbList:
		// list [kind] [--param value ...]
		if len(tokens) > 0 {
			kind := strings.ToLower(tokens[0])
			if kind == "workflows" || kind == "workflow" {
				cmd.Target = Target{Kind: "workflow"}
				tokens = tokens[1:]
			} else if kind == "jobs" || kind == "job" {
				cmd.Target = Target{Kind: "job"}
				tokens = tokens[1:]
			} else if kind == "tools" || kind == "tool" {
				cmd.Target = Target{Kind: "tool"}
				tokens = tokens[1:]
			} else if kind == "prompts" || kind == "prompt" {
				cmd.Target = Target{Kind: "prompt"}
				tokens = tokens[1:]
			} else if !strings.HasPrefix(kind, "--") {
				cmd.Target = parseTarget(kind)
				tokens = tokens[1:]
			}
		}
		cmd.Params = parseParams(tokens)
		return cmd, nil
	}

	// For run, status, cancel, restart — next token is the target.
	if len(tokens) == 0 {
		return Command{}, fmt.Errorf("%s requires a target", cmd.Verb)
	}

	cmd.Target = parseTarget(tokens[0])
	tokens = tokens[1:]
	cmd.Params = parseParams(tokens)

	return cmd, nil
}

// parseTarget parses "kind/name" or just "name" (with kind inference).
func parseTarget(s string) Target {
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		return Target{Kind: parts[0], Name: parts[1]}
	}

	// Infer kind from ID prefix.
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "wf-"):
		return Target{Kind: "workflow", Name: s}
	case strings.HasPrefix(lower, "job-"):
		return Target{Kind: "job", Name: s}
	default:
		// Ambiguous — leave kind empty for the router to resolve.
		return Target{Name: s}
	}
}

// parseParams extracts --key value pairs from remaining tokens.
func parseParams(tokens []string) map[string]any {
	params := make(map[string]any)
	for i := 0; i < len(tokens); i++ {
		if strings.HasPrefix(tokens[i], "--") {
			key := strings.TrimPrefix(tokens[i], "--")
			if key == "" {
				continue
			}
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
				params[key] = tokens[i+1]
				i++
			} else {
				params[key] = true
			}
		}
	}
	return params
}

// tokenize splits text into tokens, respecting quoted strings.
func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inQuote {
			if ch == quoteChar {
				inQuote = false
				// Include the content (without quotes) as a single token.
				tokens = append(tokens, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		} else if ch == '"' || ch == '\'' {
			inQuote = true
			quoteChar = ch
			// Flush any accumulated non-quote content.
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
