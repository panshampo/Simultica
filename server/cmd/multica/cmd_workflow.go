package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Work with runtime workflows",
}

var workflowSubmitCmd = &cobra.Command{
	Use:   "submit <issue-id>",
	Short: "Submit a runtime workflow definition for an issue (read JSON from stdin)",
	Args:  exactArgs(1),
	RunE:  runWorkflowSubmit,
}

func init() {
	workflowCmd.AddCommand(workflowSubmitCmd)
	workflowSubmitCmd.Flags().Bool("definition-stdin", false, "Read the runtime workflow JSON from stdin (recommended)")
	workflowSubmitCmd.Flags().String("definition", "", "Inline runtime workflow JSON (prefer --definition-stdin)")
	workflowSubmitCmd.Flags().Bool("initial-state-stdin", false, "Read initial_state JSON from stdin (mutually exclusive with --definition-stdin)")
	workflowSubmitCmd.Flags().String("initial-state", "", "Inline initial_state JSON")
	workflowSubmitCmd.Flags().String("output", "json", "Output format: json or table")
}

// buildSubmitRuntimeWorkflowBody validates the definition JSON and returns the
// request body. initialStateRaw may be empty (defaults to {}).
func buildSubmitRuntimeWorkflowBody(definitionRaw, initialStateRaw string) (map[string]any, error) {
	var def any
	if err := json.Unmarshal([]byte(definitionRaw), &def); err != nil {
		return nil, fmt.Errorf("--definition must be valid JSON: %w", err)
	}
	body := map[string]any{"definition": def}
	if initialStateRaw == "" {
		body["initial_state"] = map[string]any{}
	} else {
		var initial any
		if err := json.Unmarshal([]byte(initialStateRaw), &initial); err != nil {
			return nil, fmt.Errorf("--initial-state must be valid JSON: %w", err)
		}
		body["initial_state"] = initial
	}
	return body, nil
}

func runWorkflowSubmit(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		if _, err := requireWorkspaceID(cmd); err != nil {
			return err
		}
	}

	fromStdin, _ := cmd.Flags().GetBool("definition-stdin")
	definitionRaw, _ := cmd.Flags().GetString("definition")
	if fromStdin {
		buf, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read --definition-stdin: %w", err)
		}
		definitionRaw = string(buf)
	}
	if definitionRaw == "" {
		return fmt.Errorf("provide the workflow via --definition-stdin or --definition")
	}

	initialStateRaw, _ := cmd.Flags().GetString("initial-state")
	if v, _ := cmd.Flags().GetBool("initial-state-stdin"); v {
		buf, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read --initial-state-stdin: %w", err)
		}
		initialStateRaw = string(buf)
	}

	body, err := buildSubmitRuntimeWorkflowBody(definitionRaw, initialStateRaw)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("workspace_id", client.WorkspaceID)
	path := "/api/issues/" + args[0] + "/runtime-workflows?" + params.Encode()

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, path, body, &result); err != nil {
		return fmt.Errorf("submit runtime workflow: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}
