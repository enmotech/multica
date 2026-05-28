package skillfrontmatter

import (
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
