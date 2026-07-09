package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildSubmitRuntimeWorkflowBody(t *testing.T) {
	def := `{"meta":{"name":"x"},"nodes":[{"id":"a","type":"subissue","agent":"u"}],"routing":[{"from":"START","to":"a"}]}`
	body, err := buildSubmitRuntimeWorkflowBody(def, "")
	if err != nil {
		t.Fatalf("buildSubmitRuntimeWorkflowBody: %v", err)
	}
	if _, ok := body["definition"]; !ok {
		t.Fatalf("body missing definition")
	}
	if _, ok := body["initial_state"]; !ok {
		t.Fatalf("body missing initial_state default")
	}
}

func TestBuildSubmitRuntimeWorkflowBodyRejectsInvalidJSON(t *testing.T) {
	_, err := buildSubmitRuntimeWorkflowBody("{not json", "")
	if err == nil || !strings.Contains(err.Error(), "definition") {
		t.Fatalf("expected definition JSON error, got %v", err)
	}
}

func TestBuildSubmitRuntimeWorkflowBodyInitialState(t *testing.T) {
	def := `{"meta":{"name":"x"},"nodes":[{"id":"a","type":"subissue","agent":"u"}],"routing":[{"from":"START","to":"a"}]}`

	// Empty initialStateRaw defaults initial_state to an empty map.
	body, err := buildSubmitRuntimeWorkflowBody(def, "")
	if err != nil {
		t.Fatalf("buildSubmitRuntimeWorkflowBody: %v", err)
	}
	got, ok := body["initial_state"].(map[string]any)
	if !ok {
		t.Fatalf("initial_state is not a map, got %T", body["initial_state"])
	}
	if !reflect.DeepEqual(got, map[string]any{}) {
		t.Fatalf("expected empty map for default initial_state, got %#v", got)
	}

	// Provided valid JSON is parsed into the body.
	body, err = buildSubmitRuntimeWorkflowBody(def, `{"foo":"bar"}`)
	if err != nil {
		t.Fatalf("buildSubmitRuntimeWorkflowBody with initial state: %v", err)
	}
	if !reflect.DeepEqual(body["initial_state"], map[string]any{"foo": "bar"}) {
		t.Fatalf("expected parsed initial_state, got %#v", body["initial_state"])
	}
}

func TestBuildSubmitRuntimeWorkflowBodyRejectsInvalidInitialState(t *testing.T) {
	def := `{"meta":{"name":"x"},"nodes":[{"id":"a","type":"subissue","agent":"u"}],"routing":[{"from":"START","to":"a"}]}`
	_, err := buildSubmitRuntimeWorkflowBody(def, "{bad json")
	if err == nil || !strings.Contains(err.Error(), "initial-state") {
		t.Fatalf("expected initial-state JSON error, got %v", err)
	}
}

func TestWorkflowSubmitCommandIsDeprecated(t *testing.T) {
	if !strings.Contains(workflowSubmitCmd.Short, "Deprecated") {
		t.Fatalf("workflow submit short help must mark issue-first submit as deprecated, got %q", workflowSubmitCmd.Short)
	}
	if !strings.Contains(workflowSubmitCmd.Long, "debug/compatibility") {
		t.Fatalf("workflow submit long help must explain debug/compatibility scope, got %q", workflowSubmitCmd.Long)
	}
}

func TestBuildWorkflowCaseCreateRequest(t *testing.T) {
	path, body := buildWorkflowCaseCreateRequest("workspace-1", "issue-1")
	if path != "/api/issues/issue-1/workflow-cases?workspace_id=workspace-1" {
		t.Fatalf("path = %q", path)
	}
	if len(body) != 0 {
		t.Fatalf("body = %#v, want empty create body", body)
	}
}

func TestBuildWorkflowCaseDefinitionDraftBody(t *testing.T) {
	body, err := buildWorkflowCaseDefinitionDraftBody(`{"meta":{"name":"x"}}`)
	if err != nil {
		t.Fatalf("buildWorkflowCaseDefinitionDraftBody: %v", err)
	}
	if _, ok := body["draft_json"]; !ok {
		t.Fatalf("body missing draft_json: %#v", body)
	}
}

func TestBuildWorkflowCaseRunStartBody(t *testing.T) {
	body, err := buildWorkflowCaseRunStartBody("Approach A", "")
	if err != nil {
		t.Fatalf("buildWorkflowCaseRunStartBody: %v", err)
	}
	if body["label"] != "Approach A" {
		t.Fatalf("label = %#v", body["label"])
	}
	if _, ok := body["run_kind"]; ok {
		t.Fatalf("run_kind must not be present: %#v", body)
	}
	if _, ok := body["definition_version_id"]; ok {
		t.Fatalf("run start body must not include definition_version_id: %#v", body)
	}
	if !reflect.DeepEqual(body["initial_state"], map[string]any{}) {
		t.Fatalf("initial_state = %#v", body["initial_state"])
	}
}

func TestBuildWorkflowCaseRunStartBodyDefaultsPrimaryWithoutLabel(t *testing.T) {
	body, err := buildWorkflowCaseRunStartBody("", "")
	if err != nil {
		t.Fatalf("buildWorkflowCaseRunStartBody: %v", err)
	}
	if _, ok := body["run_kind"]; ok {
		t.Fatalf("run_kind must not be present: %#v", body)
	}
	if _, ok := body["label"]; ok {
		t.Fatalf("empty label must be omitted: %#v", body)
	}
}

func TestWorkflowCaseDefinitionPublishCommandRegistered(t *testing.T) {
	if workflowCaseDefinitionPublishCmd.Use != "publish <case-id>" {
		t.Fatalf("publish command Use = %q", workflowCaseDefinitionPublishCmd.Use)
	}
	for _, c := range workflowCaseDefinitionCmd.Commands() {
		if c.Name() == "confirm" {
			t.Fatalf("definition confirm command must be removed in favor of publish")
		}
	}
}

func TestWorkflowCaseDeleteCommandRegistered(t *testing.T) {
	found := false
	for _, c := range workflowCaseCmd.Commands() {
		if c.Name() == "delete" {
			found = true
		}
	}
	if !found {
		t.Fatalf("workflow-case delete command must be registered")
	}
	if workflowCaseDeleteCmd.Flags().Lookup("yes") == nil {
		t.Fatalf("workflow-case delete must expose a --yes confirmation flag")
	}
}

func TestWorkflowCaseDeleteRequiresConfirmation(t *testing.T) {
	if err := workflowCaseDeleteCmd.Flags().Set("yes", "false"); err != nil {
		t.Fatalf("set yes flag: %v", err)
	}
	err := runWorkflowCaseDelete(workflowCaseDeleteCmd, []string{"case-123"})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected refusal without --yes, got %v", err)
	}
}
