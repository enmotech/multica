package skillfrontmatter

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatterData struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Parse extracts the name and description from YAML frontmatter in a SKILL.md
// file. Returns empty strings if the content has no valid frontmatter block.
func Parse(content string) (name, description string) {
	if !strings.HasPrefix(content, "---") {
		return "", ""
	}
	rest := content[3:]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	fm, _, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", ""
	}

	var data frontmatterData
	if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
		return "", ""
	}
	return data.Name, data.Description
}
