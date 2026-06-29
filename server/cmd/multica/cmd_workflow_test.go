package main

import (
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
