package agent

import "context"

// dsCopilotBackend runs ByteDance DS Copilot through the Claude Code
// stream-json protocol. DS Copilot owns model selection through its launcher
// and CCR config, so Multica must not append a second --model flag.
type dsCopilotBackend struct {
	cfg Config
}

func init() {
	RegisterBackend("ds-copilot", func(cfg Config) (Backend, error) {
		return &dsCopilotBackend{cfg: cfg}, nil
	})
	RegisterLaunchHeader("ds-copilot", "ds-copilot (stream-json)")
}

func (b *dsCopilotBackend) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	opts.Model = ""
	return (&claudeBackend{cfg: b.cfg}).execute(ctx, prompt, opts, claudeExecutionOptions{
		defaultExecutable: "ds-copilot",
		agentLabel:        "ds-copilot",
	})
}
