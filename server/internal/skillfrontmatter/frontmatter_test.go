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
		{
			name:     "description with quoted colon",
			input:    "---\nname: quoted-skill\ndescription: \"Core analysis: TNS error checking\"\n---\n",
			wantName: "quoted-skill",
			wantDesc: "Core analysis: TNS error checking",
		},
		{
			name:     "description with unquoted colon",
			input:    "---\nname: test-skill\ndescription: Some tool. Core analysis: TNS error checking.\n---\n",
			wantName: "test-skill",
			wantDesc: "Some tool. Core analysis: TNS error checking.",
		},
		{
			name:     "nested keys do not overwrite top-level name",
			input:    "---\nname: top-level\nconfig:\n  name: nested\ndescription: desc\n---\n",
			wantName: "top-level",
			wantDesc: "desc",
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
	tests := []struct {
		name     string
		input    string
		wantName string
		wantDesc string
		wantBody string
	}{
		{
			name:     "standard frontmatter",
			input:    "---\nname: Code Reviewer\ndescription: Reviews code\n---\n\nSkill body",
			wantName: "Code Reviewer",
			wantDesc: "Reviews code",
			wantBody: "\nSkill body",
		},
		{
			name:     "no frontmatter returns full content as body",
			input:    "Just content\nNo frontmatter",
			wantName: "",
			wantDesc: "",
			wantBody: "Just content\nNo frontmatter",
		},
		{
			name:     "description with unquoted colon",
			input:    "---\nname: test-skill\ndescription: Some tool. Core analysis: TNS error checking.\n---\n\nBody",
			wantName: "test-skill",
			wantDesc: "Some tool. Core analysis: TNS error checking.",
			wantBody: "\nBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotDesc, gotBody := ParseBody(tt.input)
			if gotName != tt.wantName {
				t.Errorf("ParseBody() name = %q, want %q", gotName, tt.wantName)
			}
			if gotDesc != tt.wantDesc {
				t.Errorf("ParseBody() description = %q, want %q", gotDesc, tt.wantDesc)
			}
			if gotBody != tt.wantBody {
				t.Errorf("ParseBody() body = %q, want %q", gotBody, tt.wantBody)
			}
		})
	}
}

func TestFrontmatterBody(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOk bool
	}{
		{
			name:   "standard frontmatter",
			input:  "---\nname: Code Reviewer\ndescription: Reviews code\n---\n\nSkill body",
			want:   "name: Code Reviewer\ndescription: Reviews code",
			wantOk: true,
		},
		{
			name:   "no frontmatter",
			input:  "Just content\nNo frontmatter",
			want:   "",
			wantOk: false,
		},
		{
			name:   "unclosed frontmatter",
			input:  "---\nname: Foo",
			want:   "",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := FrontmatterBody(tt.input)
			if ok != tt.wantOk {
				t.Errorf("FrontmatterBody() ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("FrontmatterBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "has frontmatter with name",
			input: "---\nname: Code Reviewer\n---\n",
			want:  true,
		},
		{
			name:  "no frontmatter",
			input: "Just content",
			want:  false,
		},
		{
			name:  "empty name",
			input: "---\nname:\n---\n",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasFrontmatter(tt.input)
			if got != tt.want {
				t.Errorf("HasFrontmatter() = %v, want %v", got, tt.want)
			}
		})
	}
}
