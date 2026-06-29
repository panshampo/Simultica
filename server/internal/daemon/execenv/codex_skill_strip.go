package execenv

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// stripSkillsConfigEntries removes every `[[skills.config]]` array-of-tables
// block from the given config.toml content.
//
// Background: Codex Desktop writes one `[[skills.config]]` entry per skill it
// knows about — file-backed skills get a `path = "..."` field, while
// plugin-backed skills (e.g. `name = "superpowers:brainstorming"`) only get a
// `name`. Codex CLI 0.114's TOML deserializer treats `path` as a required
// field, so it rejects the plugin entries with `missing field path` and
// refuses to start. Multica copies the user's `~/.codex/config.toml` verbatim
// into each task's isolated codex-home, which propagates the broken entries
// into the per-task config and blocks `codex thread/start`.
//
// Stripping the whole `[[skills.config]]` array sidesteps the issue: Multica
// writes the agent's currently assigned skills directly to
// `codex-home/skills/<name>/SKILL.md`, and Codex auto-discovers them from
// that directory. The user-level skill registry is irrelevant to a per-task
// run, so dropping it is both safe and the right scope of isolation.
//
// Lines outside `[[skills.config]]` blocks are preserved untouched.
func stripSkillsConfigEntries(content string) string {
	if !strings.Contains(content, "[[skills.config]]") {
		return content
	}

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inSkillsConfig := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// A new TOML header always closes the current `[[skills.config]]`
		// block, regardless of whether it's another entry of the same array
		// or a different table.
		if strings.HasPrefix(trimmed, "[") {
			if trimmed == "[[skills.config]]" {
				inSkillsConfig = true
				continue
			}
			inSkillsConfig = false
			out = append(out, line)
			continue
		}

		if inSkillsConfig {
			continue
		}
		out = append(out, line)
	}

	stripped := strings.Join(out, "\n")
	// Collapse the trailing blank-line cluster that the removal can leave
	// behind so repeated copies don't grow the file unboundedly.
	stripped = strings.TrimRight(stripped, "\n") + "\n"
	if strings.TrimSpace(stripped) == "" {
		return ""
	}
	return stripped
}

var (
	// Top-level `service_tier` values outside the Codex CLI's supported
	// variants are accepted or written by some host-level tools, but current
	// Codex CLI releases reject them while parsing the per-task app-server
	// config with:
	// `unknown variant "...", expected "fast" or "flex"`.
	//
	// Multica should not copy a host-global preference that makes the
	// isolated task config unparseable. We strip only top-level incompatible
	// variants and leave explicit valid values (`fast` / `flex`) untouched so
	// user intent survives when compatible.
	rootServiceTierRe = regexp.MustCompile(`^\s*service_tier\s*=\s*(?:"([^"]+)"|'([^']+)')\s*(?:#.*)?$`)
)

// stripIncompatibleCodexConfigEntries removes root-level config lines known
// to be rejected by the Codex CLI versions Multica supports, while preserving
// unrelated content and nested table keys.
func stripIncompatibleCodexConfigEntries(content string) string {
	if !strings.Contains(content, `service_tier`) {
		return content
	}

	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	currentTable := "" // empty = TOML root
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			currentTable = trimmed
			out = append(out, line)
			continue
		}
		if currentTable == "" {
			matches := rootServiceTierRe.FindStringSubmatch(line)
			value := ""
			if len(matches) == 3 {
				value = matches[1]
				if value == "" {
					value = matches[2]
				}
			}
			if value != "" && value != "fast" && value != "flex" {
				changed = true
				continue
			}
		}
		out = append(out, line)
	}
	if !changed {
		return content
	}
	stripped := strings.Join(out, "\n")
	stripped = strings.TrimRight(stripped, "\n") + "\n"
	if strings.TrimSpace(stripped) == "" {
		return ""
	}
	return stripped
}

// sanitizeCopiedCodexConfig rewrites the per-task config.toml in place,
// dropping `[[skills.config]]` entries and root-level incompatible settings
// inherited from the shared `~/.codex/config.toml`. No-op if the file
// doesn't exist or doesn't change.
func sanitizeCopiedCodexConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config.toml: %w", err)
	}
	stripped := stripSkillsConfigEntries(string(data))
	stripped = stripIncompatibleCodexConfigEntries(stripped)
	if stripped == string(data) {
		return nil
	}
	if err := os.WriteFile(configPath, []byte(stripped), 0o644); err != nil {
		return fmt.Errorf("write config.toml: %w", err)
	}
	return nil
}
