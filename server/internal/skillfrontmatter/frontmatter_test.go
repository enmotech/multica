package skillfrontmatter

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantDesc string
	}{
		{
			name:     "standard frontmatter",
			input:    "---\nname: Code Reviewer\ndescription: Reviews code\n---\n\nSkill body",
			wantName: "Code Reviewer",
			wantDesc: "Reviews code",
		},
		{
			name:     "no frontmatter",
			input:    "Just content\nNo frontmatter",
			wantName: "",
			wantDesc: "",
		},
		{
			name:     "unclosed frontmatter",
			input:    "---\nname: Foo",
			wantName: "",
			wantDesc: "",
		},
		{
			name:     "empty frontmatter",
			input:    "---\n---\n\nBody",
			wantName: "",
			wantDesc: "",
		},
		{
			name:     "name only",
			input:    "---\nname: Only Name\n---\n",
			wantName: "Only Name",
			wantDesc: "",
		},
		{
			name:     "quoted values",
			input:    "---\nname: \"Quoted Name\"\ndescription: 'Single quoted'\n---\n",
			wantName: "Quoted Name",
			wantDesc: "Single quoted",
		},
		{
			name:     "extra fields ignored",
			input:    "---\nname: My Skill\nversion: 1.0\nauthor: test\n---\n",
			wantName: "My Skill",
			wantDesc: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotDesc := Parse(tt.input)
			if gotName != tt.wantName {
				t.Errorf("Parse() name = %q, want %q", gotName, tt.wantName)
			}
			if gotDesc != tt.wantDesc {
				t.Errorf("Parse() description = %q, want %q", gotDesc, tt.wantDesc)
			}
		})
	}
}

func TestParseBody(t *testing.T) {
	input := "---\nname: Test\n---\n\nThis is the body."
	name, desc, body := ParseBody(input)
	if name != "Test" {
		t.Errorf("ParseBody() name = %q, want %q", name, "Test")
	}
	if desc != "" {
		t.Errorf("ParseBody() description = %q, want empty", desc)
	}
	wantBody := "This is the body."
	if strings.TrimSpace(body) != wantBody {
		t.Errorf("ParseBody() body = %q, want %q", strings.TrimSpace(body), wantBody)
	}
}

func TestBuild(t *testing.T) {
	result := Build("My Skill", "A description", "Body content")
	if !strings.HasPrefix(result, "---\n") {
		t.Error("Build() should start with ---")
	}
	if !strings.Contains(result, "name: My Skill") {
		t.Error("Build() should contain name")
	}
	if !strings.Contains(result, "description: A description") {
		t.Error("Build() should contain description")
	}
	if !strings.Contains(result, "---\n\nBody content") {
		t.Error("Build() should contain body after closing ---")
	}
}

func TestBuildEmptyDescription(t *testing.T) {
	result := Build("My Skill", "", "Body")
	if strings.Contains(result, "description:") {
		t.Error("Build() should not include description when empty")
	}
}
