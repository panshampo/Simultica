package agent

import (
	"context"
)

// traexBackend implements Backend for the Traex CLI (a fork of Codex that
// shares the same `app-server --listen stdio://` JSON-RPC entry point and
// `AGENTS.md` runtime-config convention). The transport, thread/turn
// protocol, and reasoning-effort vocabulary all match Codex bit-for-bit
// against traecli 0.200.7, so the backend simply delegates to codexBackend
// with the executable defaulted to `traex`.
//
// Two intentional simplifications relative to codexBackend:
//
//   - No per-task TRAE_HOME isolation. The daemon does not seed a per-task
//     copy of `~/.trae/`; Traex reads the user's shared home directly. That
//     keeps installation friction to "the user already has `traex` working"
//     and matches how OpenClaw / Antigravity also operate without an
//     isolated home.
//   - No managed MCP materialisation. CODEX_HOME is never set on the child
//     env, so the MCP-config branch in codexBackend.Execute is skipped.
//     Operators who need MCP for Traex configure it through the user-level
//     `~/.trae/traecli.toml`. This can be lifted later by symlinking a
//     per-task TRAE_HOME analogous to codex_home.go.
type traexBackend struct {
	cfg Config
}

func init() {
	RegisterBackend("traex", func(cfg Config) (Backend, error) {
		return &traexBackend{cfg: cfg}, nil
	})
	RegisterLaunchHeader("traex", "traex app-server")
}

func (b *traexBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	cfg := b.cfg
	if cfg.ExecutablePath == "" {
		cfg.ExecutablePath = "traex"
	}
	inner := &codexBackend{cfg: cfg}
	return inner.Execute(ctx, prompt, opts)
}
