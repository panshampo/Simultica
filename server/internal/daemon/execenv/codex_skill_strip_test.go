package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripSkillsConfigEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no skills config — returned unchanged",
			in:   "model = \"o3\"\n",
			want: "model = \"o3\"\n",
		},
		{
			name: "drops well-formed file-backed entry",
			in: `model = "o3"

[[skills.config]]
path = "/Users/x/SKILL.md"
enabled = true
`,
			want: `model = "o3"
`,
		},
		{
			name: "drops plugin entry that lacks path",
			in: `[[skills.config]]
name = "superpowers:brainstorming"
enabled = false

[profiles.default]
model = "o3"
`,
			want: `[profiles.default]
model = "o3"
`,
		},
		{
			name: "drops a mix of consecutive entries and preserves surrounding tables",
			in: `model = "o3"

[[skills.config]]
path = "/Users/x/SKILL.md"
enabled = false

[[skills.config]]
path = "/Users/y/SKILL.md"
enabled = false

[[skills.config]]
name = "superpowers:brainstorming"
enabled = false

[profiles.default]
model = "o3"

[mcp_servers.foo]
command = "foo"
`,
			want: `model = "o3"

[profiles.default]
model = "o3"

[mcp_servers.foo]
command = "foo"
`,
		},
		{
			name: "skills.config at EOF",
			in: `model = "o3"

[[skills.config]]
name = "superpowers:dispatching-parallel-agents"
enabled = false
`,
			want: `model = "o3"
`,
		},
		{
			name: "preserves unrelated [skills] table (single brackets)",
			in: `[skills]
discovery_path = "skills"
`,
			want: `[skills]
discovery_path = "skills"
`,
		},
		{
			name: "fully empty after strip returns empty string",
			in: `[[skills.config]]
name = "x"
enabled = false
`,
			want: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripSkillsConfigEntries(tt.in)
			if got != tt.want {
				t.Errorf("stripSkillsConfigEntries result mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tt.want)
			}
		})
	}
}

func TestStripIncompatibleCodexConfigEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "root service_tier default is stripped",
			in: `model = "gpt-5.5"
service_tier = "default"
approval_policy = "never"
`,
			want: `model = "gpt-5.5"
approval_policy = "never"
`,
		},
		{
			name: "root service_tier priority is stripped",
			in: `model = "gpt-5.5"
service_tier = "priority"
approval_policy = "never"
`,
			want: `model = "gpt-5.5"
approval_policy = "never"
`,
		},
		{
			name: "valid root service_tier is preserved",
			in: `model = "gpt-5.5"
service_tier = "fast"
`,
			want: `model = "gpt-5.5"
service_tier = "fast"
`,
		},
		{
			name: "valid root service_tier flex is preserved",
			in: `model = "gpt-5.5"
service_tier = "flex"
`,
			want: `model = "gpt-5.5"
service_tier = "flex"
`,
		},
		{
			name: "nested service_tier key is preserved",
			in: `[profiles.default]
service_tier = "priority"
`,
			want: `[profiles.default]
service_tier = "priority"
`,
		},
		{
			name: "commented root default line is preserved",
			in: `model = "gpt-5.5"
# service_tier = "default"
`,
			want: `model = "gpt-5.5"
# service_tier = "default"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripIncompatibleCodexConfigEntries(tt.in)
			if got != tt.want {
				t.Errorf("stripIncompatibleCodexConfigEntries mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tt.want)
			}
		})
	}
}

func TestSanitizeCopiedCodexConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	original := `model = "o3"
service_tier = "default"

[[skills.config]]
name = "superpowers:brainstorming"
enabled = false

[[skills.config]]
path = "/Users/x/SKILL.md"
enabled = true

[profiles.default]
model = "o3"
`
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if err := sanitizeCopiedCodexConfig(configPath); err != nil {
		t.Fatalf("sanitizeCopiedCodexConfig failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "[[skills.config]]") {
		t.Errorf("expected all [[skills.config]] entries to be removed, got:\n%s", got)
	}
	if strings.Contains(got, `service_tier = "default"`) {
		t.Errorf("expected incompatible root service_tier to be removed, got:\n%s", got)
	}
	if !strings.Contains(got, `[profiles.default]`) {
		t.Errorf("unrelated tables should be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, `model = "o3"`) {
		t.Errorf("top-level keys should be preserved, got:\n%s", got)
	}
}

func TestSanitizeCopiedCodexConfigNoop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	original := "model = \"o3\"\n"
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	infoBefore, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	if err := sanitizeCopiedCodexConfig(configPath); err != nil {
		t.Fatalf("sanitizeCopiedCodexConfig failed: %v", err)
	}

	infoAfter, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Errorf("file should not be rewritten when there is nothing to strip")
	}
	data, _ := os.ReadFile(configPath)
	if string(data) != original {
		t.Errorf("content drifted: got %q, want %q", data, original)
	}
}

func TestSanitizeCopiedCodexConfigMissingFile(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "does-not-exist.toml")
	if err := sanitizeCopiedCodexConfig(missing); err != nil {
		t.Errorf("missing file should be a no-op, got error: %v", err)
	}
}
