package skillfrontmatter

import (
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatterData struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Parse extracts name and description from YAML frontmatter in a SKILL.md file.
// Returns empty strings if the content has no valid frontmatter block.
func Parse(content string) (name, description string) {
	name, description, _ = ParseBody(content)
	return
}

// ParseBody extracts frontmatter fields and returns the body after the closing
// --- delimiter. If there is no valid frontmatter, it returns ("", "", content).
func ParseBody(content string) (name, description, body string) {
	fm, bodyStart, ok := frontmatterBounds(content)
	if !ok {
		return "", "", content
	}

	var data frontmatterData
	if err := yaml.Unmarshal([]byte(fm), &data); err != nil {
		data.Name, data.Description = extractFromRaw(fm)
	}

	if bodyStart < len(content) {
		body = content[bodyStart:]
	}
	return data.Name, data.Description, body
}

// frontmatterBounds returns the raw YAML frontmatter text and the byte offset
// where the body begins. Returns ok=false when no frontmatter is present.
func frontmatterBounds(content string) (fm string, bodyStart int, ok bool) {
	if !strings.HasPrefix(content, "---") {
		return "", 0, false
	}
	rest := content[3:]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", 0, false
	}
	fm = rest[:end]
	bodyStart = 3 + 1 + end + len("\n---")
	if bodyStart < len(content) && content[bodyStart] == '\n' {
		bodyStart++
	}
	return fm, bodyStart, true
}

// extractFromRaw pulls name and description directly from raw frontmatter text
// when strict YAML parsing fails (e.g. unquoted colons inside scalar values).
func extractFromRaw(fm string) (name, description string) {
	for _, line := range strings.Split(fm, "\n") {
		// Skip empty or indented lines to avoid misinterpreting nested keys
		// as top-level fields like name or description.
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			v = strings.TrimSpace(v)
			switch strings.TrimSpace(k) {
			case "name":
				name = strings.Trim(v, `"'`)
			case "description":
				description = strings.Trim(v, `"'`)
			}
		}
	}
	return
}

// HasFrontmatter reports whether content starts with a YAML frontmatter block
// that contains a non-empty "name" field.
func HasFrontmatter(content string) bool {
	name, _ := Parse(content)
	return strings.TrimSpace(name) != ""
}

// FrontmatterBody returns the raw YAML text between the opening --- and closing
// --- delimiters. Returns ("", false) when no frontmatter is present.
func FrontmatterBody(content string) (string, bool) {
	fm, _, ok := frontmatterBounds(content)
	return fm, ok
}
