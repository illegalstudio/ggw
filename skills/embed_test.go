package skills

import (
	"io/fs"
	"strings"
	"testing"
)

func TestBundledSkillContainsExpectedFiles(t *testing.T) {
	wants := []string{
		"ggw/SKILL.md",
		"ggw/agents/openai.yaml",
	}

	for _, want := range wants {
		if _, err := fs.Stat(bundled, want); err != nil {
			t.Fatalf("bundled skill missing %s: %v", want, err)
		}
	}
}

func TestBundledSkillFrontmatterDeclaresName(t *testing.T) {
	contents, err := bundled.ReadFile("ggw/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}

	text := string(contents)
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("SKILL.md does not start with YAML frontmatter:\n%.40s", text)
	}
	if !strings.Contains(text, "\nname: ggw\n") {
		t.Fatal("SKILL.md frontmatter does not declare 'name: ggw'")
	}
	if !strings.Contains(text, "\ndescription: ") {
		t.Fatal("SKILL.md frontmatter does not declare a description")
	}
}
