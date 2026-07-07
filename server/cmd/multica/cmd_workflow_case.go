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

var workflowCaseCmd = &cobra.Command{
	Use:   "workflow-case",
	Short: "Work with WorkflowCase control plane",
	Long: `Work with the WorkflowCase control plane.

WorkflowCase is the canonical owner of runtime workflow definitions and runs.
Use this command surface instead of issue-first workflow submission.`,
}

var workflowCaseCreateCmd = &cobra.Command{
	Use:   "create --entry-issue <issue-id>",
	Short: "Create a WorkflowCase for an entry issue",
	Args:  exactArgs(0),
	RunE:  runWorkflowCaseCreate,
}

var workflowCaseDefinitionCmd = &cobra.Command{
	Use:   "definition",
	Short: "Manage WorkflowCase definitions",
}

var workflowCaseDefinitionUpsertCmd = &cobra.Command{
	Use:   "upsert <case-id>",
	Short: "Upsert a WorkflowCase definition draft",
	Args:  exactArgs(1),
	RunE:  runWorkflowCaseDefinitionUpsert,
}

var workflowCaseDefinitionValidateCmd = &cobra.Command{
	Use:   "validate <case-id>",
	Short: "Validate a WorkflowCase definition draft",
	Args:  exactArgs(1),
	RunE:  runWorkflowCaseDefinitionValidate,
}

var workflowCaseDefinitionPublishCmd = &cobra.Command{
	Use:   "publish <case-id>",
	Short: "Publish the WorkflowCase draft as the online version",
	Args:  exactArgs(1),
	RunE:  runWorkflowCaseDefinitionPublish,
}

var workflowCaseRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Manage WorkflowCase runs",
}

var workflowCaseDeleteCmd = &cobra.Command{
	Use:   "delete <case-id>",
	Short: "Permanently delete a WorkflowCase and all its versions and runs",
	Long: `Permanently delete a WorkflowCase.

This hard-deletes the case together with its definition draft, all versions,
all runs, run nodes, and node events. Carrier sub-issues are detached from the
workflow but not deleted. The case must have no active run.

This action cannot be undone.`,
	Args: exactArgs(1),
	RunE: runWorkflowCaseDelete,
}

var workflowCaseRunStartCmd = &cobra.Command{
	Use:   "start <case-id>",
	Short: "Start a WorkflowRun from a WorkflowCase",
	Args:  exactArgs(1),
	RunE:  runWorkflowCaseRunStart,
}

func init() {
	workflowCaseCmd.AddCommand(workflowCaseCreateCmd)
	workflowCaseCmd.AddCommand(workflowCaseDefinitionCmd)
	workflowCaseCmd.AddCommand(workflowCaseRunCmd)
	workflowCaseCmd.AddCommand(workflowCaseDeleteCmd)

	workflowCaseDeleteCmd.Flags().Bool("yes", false, "Confirm permanent deletion without an interactive prompt")

	workflowCaseCreateCmd.Flags().String("entry-issue", "", "Entry issue ID for the WorkflowCase")
	workflowCaseCreateCmd.Flags().String("output", "json", "Output format: json")

	workflowCaseDefinitionCmd.AddCommand(workflowCaseDefinitionUpsertCmd)
	workflowCaseDefinitionCmd.AddCommand(workflowCaseDefinitionValidateCmd)
	workflowCaseDefinitionCmd.AddCommand(workflowCaseDefinitionPublishCmd)

	workflowCaseDefinitionUpsertCmd.Flags().Bool("definition-stdin", false, "Read the workflow definition JSON from stdin")
	workflowCaseDefinitionUpsertCmd.Flags().String("definition", "", "Inline workflow definition JSON (prefer --definition-stdin)")
	workflowCaseDefinitionUpsertCmd.Flags().Bool("source-templates-stdin", false, "Read source_templates JSON from stdin (mutually exclusive with --definition-stdin)")
	workflowCaseDefinitionUpsertCmd.Flags().String("source-templates", "", "Inline source_templates JSON")
	workflowCaseDefinitionUpsertCmd.Flags().String("output", "json", "Output format: json")

	workflowCaseDefinitionValidateCmd.Flags().String("output", "json", "Output format: json")

	workflowCaseDefinitionPublishCmd.Flags().String("note", "", "Optional publish note")
	workflowCaseDefinitionPublishCmd.Flags().String("output", "json", "Output format: json")

	workflowCaseRunCmd.AddCommand(workflowCaseRunStartCmd)
	workflowCaseRunStartCmd.Flags().String("run-kind", "primary", "Run kind: primary|experiment|shadow|replay|debug")
	workflowCaseRunStartCmd.Flags().String("label", "", "Optional run label")
	workflowCaseRunStartCmd.Flags().Bool("initial-state-stdin", false, "Read initial_state JSON from stdin")
	workflowCaseRunStartCmd.Flags().String("initial-state", "", "Inline initial_state JSON")
	workflowCaseRunStartCmd.Flags().String("output", "json", "Output format: json")
}

func buildWorkflowCaseCreateRequest(workspaceID, entryIssueID string) (string, map[string]any) {
	params := url.Values{}
	params.Set("workspace_id", workspaceID)
	return "/api/issues/" + entryIssueID + "/workflow-cases?" + params.Encode(), map[string]any{}
}

func buildWorkflowCaseDefinitionDraftBody(definitionRaw string) (map[string]any, error) {
	var def any
	if err := json.Unmarshal([]byte(definitionRaw), &def); err != nil {
		return nil, fmt.Errorf("--definition must be valid JSON: %w", err)
	}
	return map[string]any{
		"draft_json":       def,
		"source_templates": []any{},
	}, nil
}

func buildWorkflowCaseDefinitionDraftBodyWithSourceTemplates(definitionRaw, sourceTemplatesRaw string) (map[string]any, error) {
	body, err := buildWorkflowCaseDefinitionDraftBody(definitionRaw)
	if err != nil {
		return nil, err
	}
	if sourceTemplatesRaw == "" {
		return body, nil
	}
	var templates any
	if err := json.Unmarshal([]byte(sourceTemplatesRaw), &templates); err != nil {
		return nil, fmt.Errorf("--source-templates must be valid JSON: %w", err)
	}
	body["source_templates"] = templates
	return body, nil
}

func buildWorkflowCaseRunStartBody(runKind, label, initialStateRaw string) (map[string]any, error) {
	body := map[string]any{"run_kind": runKind}
	if label != "" {
		body["label"] = label
	}
	if initialStateRaw == "" {
		body["initial_state"] = map[string]any{}
		return body, nil
	}
	var initial any
	if err := json.Unmarshal([]byte(initialStateRaw), &initial); err != nil {
		return nil, fmt.Errorf("--initial-state must be valid JSON: %w", err)
	}
	body["initial_state"] = initial
	return body, nil
}

func runWorkflowCaseCreate(cmd *cobra.Command, _ []string) error {
	client, err := workflowCaseAPIClient(cmd)
	if err != nil {
		return err
	}
	entryIssueID, _ := cmd.Flags().GetString("entry-issue")
	if entryIssueID == "" {
		return fmt.Errorf("--entry-issue is required")
	}
	if err := requireJSONOutput(cmd); err != nil {
		return err
	}

	path, body := buildWorkflowCaseCreateRequest(client.WorkspaceID, entryIssueID)
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, path, body, &result); err != nil {
		return fmt.Errorf("create workflow case: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runWorkflowCaseDefinitionUpsert(cmd *cobra.Command, args []string) error {
	client, err := workflowCaseAPIClient(cmd)
	if err != nil {
		return err
	}
	if err := requireJSONOutput(cmd); err != nil {
		return err
	}

	definitionRaw, sourceTemplatesRaw, err := workflowCaseDefinitionInputs(cmd)
	if err != nil {
		return err
	}
	body, err := buildWorkflowCaseDefinitionDraftBodyWithSourceTemplates(definitionRaw, sourceTemplatesRaw)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("workspace_id", client.WorkspaceID)
	path := "/api/workflow-cases/" + args[0] + "/definition/draft?" + params.Encode()

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result map[string]any
	if err := client.PutJSON(ctx, path, body, &result); err != nil {
		return fmt.Errorf("upsert workflow case definition: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runWorkflowCaseDefinitionValidate(cmd *cobra.Command, args []string) error {
	client, err := workflowCaseAPIClient(cmd)
	if err != nil {
		return err
	}
	if err := requireJSONOutput(cmd); err != nil {
		return err
	}

	params := url.Values{}
	params.Set("workspace_id", client.WorkspaceID)
	path := "/api/workflow-cases/" + args[0] + "/definition/validate?" + params.Encode()

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, path, nil, &result); err != nil {
		return fmt.Errorf("validate workflow case definition: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runWorkflowCaseDefinitionPublish(cmd *cobra.Command, args []string) error {
	client, err := workflowCaseAPIClient(cmd)
	if err != nil {
		return err
	}
	if err := requireJSONOutput(cmd); err != nil {
		return err
	}

	note, _ := cmd.Flags().GetString("note")
	body := map[string]any{}
	if note != "" {
		body["note"] = note
	}
	params := url.Values{}
	params.Set("workspace_id", client.WorkspaceID)
	path := "/api/workflow-cases/" + args[0] + "/definition/publish?" + params.Encode()

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, path, body, &result); err != nil {
		return fmt.Errorf("publish workflow case definition: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runWorkflowCaseRunStart(cmd *cobra.Command, args []string) error {
	client, err := workflowCaseAPIClient(cmd)
	if err != nil {
		return err
	}
	if err := requireJSONOutput(cmd); err != nil {
		return err
	}

	runKind, _ := cmd.Flags().GetString("run-kind")
	if runKind == "" {
		runKind = "primary"
	}
	label, _ := cmd.Flags().GetString("label")
	initialStateRaw, err := workflowCaseInitialStateInput(cmd)
	if err != nil {
		return err
	}
	body, err := buildWorkflowCaseRunStartBody(runKind, label, initialStateRaw)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("workspace_id", client.WorkspaceID)
	path := "/api/workflow-cases/" + args[0] + "/runs?" + params.Encode()

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var result map[string]any
	if err := client.PostJSON(ctx, path, body, &result); err != nil {
		return fmt.Errorf("start workflow case run: %w", err)
	}
	return cli.PrintJSON(os.Stdout, result)
}

func runWorkflowCaseDelete(cmd *cobra.Command, args []string) error {
	client, err := workflowCaseAPIClient(cmd)
	if err != nil {
		return err
	}
	confirmed, _ := cmd.Flags().GetBool("yes")
	if !confirmed {
		return fmt.Errorf("refusing to delete workflow case %s without --yes; this permanently removes all its versions and runs", args[0])
	}

	params := url.Values{}
	params.Set("workspace_id", client.WorkspaceID)
	path := "/api/workflow-cases/" + args[0] + "?" + params.Encode()

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	if err := client.DeleteJSON(ctx, path); err != nil {
		return fmt.Errorf("delete workflow case: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Deleted workflow case %s\n", args[0])
	return nil
}

func workflowCaseAPIClient(cmd *cobra.Command) (*cli.APIClient, error) {
	client, err := newAPIClient(cmd)
	if err != nil {
		return nil, err
	}
	if client.WorkspaceID == "" {
		if _, err := requireWorkspaceID(cmd); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func requireJSONOutput(cmd *cobra.Command) error {
	output, _ := cmd.Flags().GetString("output")
	if output != "json" {
		return fmt.Errorf("unsupported --output %q: only json is supported", output)
	}
	return nil
}

func workflowCaseDefinitionInputs(cmd *cobra.Command) (string, string, error) {
	definitionFromStdin, _ := cmd.Flags().GetBool("definition-stdin")
	sourceTemplatesFromStdin, _ := cmd.Flags().GetBool("source-templates-stdin")
	if definitionFromStdin && sourceTemplatesFromStdin {
		return "", "", fmt.Errorf("--definition-stdin and --source-templates-stdin cannot be combined; only one value can be read from stdin")
	}

	definitionRaw, _ := cmd.Flags().GetString("definition")
	if definitionFromStdin {
		buf, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", "", fmt.Errorf("read --definition-stdin: %w", err)
		}
		definitionRaw = string(buf)
	}
	if definitionRaw == "" {
		return "", "", fmt.Errorf("provide the workflow via --definition-stdin or --definition")
	}

	sourceTemplatesRaw, _ := cmd.Flags().GetString("source-templates")
	if sourceTemplatesFromStdin {
		buf, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", "", fmt.Errorf("read --source-templates-stdin: %w", err)
		}
		sourceTemplatesRaw = string(buf)
	}
	return definitionRaw, sourceTemplatesRaw, nil
}

func workflowCaseInitialStateInput(cmd *cobra.Command) (string, error) {
	initialStateFromStdin, _ := cmd.Flags().GetBool("initial-state-stdin")
	initialStateRaw, _ := cmd.Flags().GetString("initial-state")
	if initialStateFromStdin {
		buf, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("read --initial-state-stdin: %w", err)
		}
		initialStateRaw = string(buf)
	}
	return initialStateRaw, nil
}
