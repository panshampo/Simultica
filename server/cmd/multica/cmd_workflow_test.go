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
