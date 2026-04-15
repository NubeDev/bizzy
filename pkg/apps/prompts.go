package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// promptFrontmatter is the YAML frontmatter in a prompt markdown file.
type promptFrontmatter struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Arguments   []PromptArgument `yaml:"arguments"`
}

// LoadPrompts loads all markdown prompts from an app's prompts/ directory.
func LoadPrompts(app *App) ([]Prompt, error) {
	promptsDir := filepath.Join(app.Dir, "prompts")
	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read prompts dir: %w", err)
	}

	var prompts []Prompt
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		p, err := loadPromptFile(filepath.Join(promptsDir, e.Name()), app.Name)
		if err != nil {
			return nil, fmt.Errorf("load prompt %s: %w", e.Name(), err)
		}
		prompts = append(prompts, *p)
	}
	return prompts, nil
}

func loadPromptFile(path, appName string) (*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	var fm promptFrontmatter
	var body string

	// Parse frontmatter delimited by ---
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
				return nil, fmt.Errorf("parse frontmatter: %w", err)
			}
			body = strings.TrimSpace(parts[1])
		} else {
			body = content
		}
	} else {
		body = content
	}

	// Fallback: use filename as name if not in frontmatter.
	if fm.Name == "" {
		base := filepath.Base(path)
		fm.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return &Prompt{
		Name:        fm.Name,
		Description: fm.Description,
		Arguments:   fm.Arguments,
		Body:        body,
		AppName:     appName,
	}, nil
}
