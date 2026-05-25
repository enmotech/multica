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
	_, _, name, description = parseRaw(content)
	return name, description
}

// ParseBody extracts frontmatter fields and returns the body after the closing
// --- delimiter.
func ParseBody(content string) (name, description, body string) {
	bodyStart, _, name, description := parseRaw(content)
	if bodyStart < 0 {
		return name, description, content
	}
	return name, description, content[bodyStart:]
}

func parseRaw(content string) (bodyStart int, bodyEnd int, name, description string) {
	if !strings.HasPrefix(content, "---") {
		return -1, -1, "", ""
	}
	rest := content[3:]
	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return -1, -1, "", ""
	}
	fm := rest[:end]

	var data frontmatterData
	if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
		return -1, -1, "", ""
	}

	// Body starts after the closing --- and its trailing newline
	bodyOffset := 3 + 1 + end + len("\n---")
	if bodyOffset < len(content) && content[bodyOffset] == '\n' {
		bodyOffset++
	}
	return bodyOffset, len(content), data.Name, data.Description
}

// Build creates a SKILL.md string with YAML frontmatter and body content.
// If description is empty, it is omitted from the frontmatter.
func Build(name, description, body string) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString("name: ")
	sb.WriteString(quoteYAMLString(name))
	sb.WriteByte('\n')
	if description != "" {
		sb.WriteString("description: ")
		sb.WriteString(quoteYAMLString(description))
		sb.WriteByte('\n')
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	return sb.String()
}

// quoteYAMLString produces a safe YAML scalar.
func quoteYAMLString(s string) string {
	if s == "" {
		return `""`
	}
	var node yaml.Node
	node.SetString(s)
	return node.Value
}
