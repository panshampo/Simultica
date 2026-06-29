package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewReturnsClaudeBackend(t *testing.T) {
	t.Parallel()
	b, err := New("claude", Config{ExecutablePath: "/nonexistent/claude"})
	if err != nil {
		t.Fatalf("New(claude) error: %v", err)
	}
	if _, ok := b.(*claudeBackend); !ok {
		t.Fatalf("expected *claudeBackend, got %T", b)
	}
}

func TestNewReturnsDsCopilotBackend(t *testing.T) {
	t.Parallel()
	b, err := New("ds-copilot", Config{ExecutablePath: "/nonexistent/ds-copilot"})
	if err != nil {
		t.Fatalf("New(ds-copilot) error: %v", err)
	}
	if _, ok := b.(*dsCopilotBackend); !ok {
		t.Fatalf("expected *dsCopilotBackend, got %T", b)
	}
}

func TestNewReturnsCodexBackend(t *testing.T) {
	t.Parallel()
	b, err := New("codex", Config{ExecutablePath: "/nonexistent/codex"})
	if err != nil {
		t.Fatalf("New(codex) error: %v", err)
	}
	if _, ok := b.(*codexBackend); !ok {
		t.Fatalf("expected *codexBackend, got %T", b)
	}
}

func TestNewReturnsCopilotBackend(t *testing.T) {
	t.Parallel()
	b, err := New("copilot", Config{ExecutablePath: "/nonexistent/copilot"})
	if err != nil {
		t.Fatalf("New(copilot) error: %v", err)
	}
	if _, ok := b.(*copilotBackend); !ok {
		t.Fatalf("expected *copilotBackend, got %T", b)
	}
}

func TestNewReturnsAntigravityBackend(t *testing.T) {
	t.Parallel()
	b, err := New("antigravity", Config{ExecutablePath: "/nonexistent/agy"})
	if err != nil {
		t.Fatalf("New(antigravity) error: %v", err)
	}
	if _, ok := b.(*antigravityBackend); !ok {
		t.Fatalf("expected *antigravityBackend, got %T", b)
	}
}

func TestNewRejectsUnknownType(t *testing.T) {
	t.Parallel()
	_, err := New("gpt", Config{})
	if err == nil {
		t.Fatal("expected error for unknown agent type")
	}
}

func TestNewDefaultsLogger(t *testing.T) {
	t.Parallel()
	b, _ := New("claude", Config{})
	cb := b.(*claudeBackend)
	if cb.cfg.Logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestDetectVersionFailsForMissingBinary(t *testing.T) {
	t.Parallel()
	_, err := DetectVersion(context.Background(), "/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestDetectVersionWithArgsPrependsExecutableArgs(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	dir := t.TempDir()
	fakePath := filepath.Join(dir, "runner")
	argsPath := filepath.Join(dir, "args.txt")
	launcherPath := filepath.Join(dir, "launch.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"printf '%s\\n' 'ds-copilot 1.8.2'\n"
	writeTestExecutable(t, fakePath, []byte(script))

	version, err := DetectVersionWithArgs(context.Background(), fakePath, []string{launcherPath})
	if err != nil {
		t.Fatalf("DetectVersionWithArgs: %v", err)
	}
	if version != "ds-copilot 1.8.2" {
		t.Fatalf("version = %q, want ds-copilot 1.8.2", version)
	}

	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := []string{launcherPath, "--version"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestLaunchHeaderCoversAllSupportedBackends(t *testing.T) {
	t.Parallel()

	// The factory in New() enumerates every supported agent type; LaunchHeader
	// must stay in sync so the UI preview never shows an empty skeleton for a
	// runtime the daemon actually spawns. If a new backend is added, add an
	// entry to launchHeaders in agent.go and extend this list.
	supported := []string{
		"antigravity", "claude", "codex", "copilot", "cursor", "ds-copilot",
		"gemini", "hermes", "kimi", "kiro", "openclaw", "opencode", "pi",
		"traex",
	}
	for _, t_ := range supported {
		if header := LaunchHeader(t_); header == "" {
			t.Errorf("LaunchHeader(%q) returned empty string — add it to launchHeaders", t_)
		}
	}
}

func TestDsCopilotExecuteDoesNotForwardAgentModel(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	dir := t.TempDir()
	fakePath := filepath.Join(dir, "ds-copilot")
	argsPath := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"IFS= read -r _\n" +
		"printf '%s\\n' '{\"type\":\"system\",\"session_id\":\"sess-ds\"}'\n" +
		"printf '%s\\n' '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"session_id\":\"sess-ds\",\"result\":\"done\",\"model\":\"gec,gpt-pro\",\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}'\n"
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("ds-copilot", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new ds-copilot backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{
		Model:   "claude-sonnet-4-6",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		if result.Status != "completed" {
			t.Fatalf("status = %q, error = %q", result.Status, result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}

	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for i, arg := range args {
		if arg == "--model" || arg == "claude-sonnet-4-6" {
			t.Fatalf("ds-copilot must not receive Multica model args, got arg[%d]=%q in %v", i, arg, args)
		}
	}
}

func TestDsCopilotExecutePrependsExecutableArgs(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}

	dir := t.TempDir()
	fakePath := filepath.Join(dir, "bash")
	argsPath := filepath.Join(dir, "args.txt")
	launcherPath := filepath.Join(dir, "launch.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"IFS= read -r _\n" +
		"printf '%s\\n' '{\"type\":\"system\",\"session_id\":\"sess-ds\"}'\n" +
		"printf '%s\\n' '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"session_id\":\"sess-ds\",\"result\":\"done\",\"model\":\"gec,gpt-pro\",\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}'\n"
	writeTestExecutable(t, fakePath, []byte(script))

	backend, err := New("ds-copilot", Config{
		ExecutablePath: fakePath,
		ExecutableArgs: []string{launcherPath},
		Logger:         slog.Default(),
	})
	if err != nil {
		t.Fatalf("new ds-copilot backend: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "prompt-ignored", ExecOptions{
		Model:   "claude-sonnet-4-6",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	select {
	case result, ok := <-session.Result:
		if !ok {
			t.Fatal("result channel closed without a value")
		}
		if result.Status != "completed" {
			t.Fatalf("status = %q, error = %q", result.Status, result.Error)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for result")
	}

	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(args) == 0 || args[0] != launcherPath {
		t.Fatalf("ds-copilot launcher arg missing, got %v", args)
	}
	for i, arg := range args {
		if arg == "--model" || arg == "claude-sonnet-4-6" {
			t.Fatalf("ds-copilot must not receive Multica model args, got arg[%d]=%q in %v", i, arg, args)
		}
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func TestLaunchHeaderReturnsEmptyForUnknownType(t *testing.T) {
	t.Parallel()
	if header := LaunchHeader("made-up-agent"); header != "" {
		t.Errorf("expected empty header for unknown type, got %q", header)
	}
}

func TestRunContextZeroTimeoutHasNoDeadline(t *testing.T) {
	t.Parallel()
	// A zero (or negative) timeout must NOT impose a wall-clock deadline:
	// liveness is delegated to the daemon's inactivity watchdog so an actively
	// streaming long-running session is never killed merely for running long
	// (MUL-3064).
	for _, d := range []time.Duration{0, -time.Second} {
		ctx, cancel := runContext(context.Background(), d)
		if _, ok := ctx.Deadline(); ok {
			cancel()
			t.Fatalf("runContext(%s) imposed a deadline; want none", d)
		}
		cancel()
		if ctx.Err() == nil {
			t.Fatalf("runContext(%s): context should be cancelled after cancel()", d)
		}
	}
}

func TestRunContextPositiveTimeoutHasDeadline(t *testing.T) {
	t.Parallel()
	// A positive timeout keeps the hard wall-clock deadline (the opt-in
	// absolute cap operators can still set via MULTICA_AGENT_TIMEOUT).
	ctx, cancel := runContext(context.Background(), time.Hour)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("runContext(1h) should impose a deadline")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Hour+time.Minute {
		t.Fatalf("unexpected deadline remaining: %s", remaining)
	}
}
