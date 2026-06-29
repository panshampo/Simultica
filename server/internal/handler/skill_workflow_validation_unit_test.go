package handler

import (
	"strings"
	"testing"
)

func TestValidateSkillWorkflowYAMLAcceptsLoopWithBudget(t *testing.T) {
	result := validateSkillWorkflowYAML(validWorkflowYAML())

	if !result.Valid {
		t.Fatalf("workflow should be valid, errors=%v", result.Errors)
	}
}

func TestValidateSkillWorkflowYAMLRejectsUndefinedConditionField(t *testing.T) {
	result := validateSkillWorkflowYAML(`
meta:
  name: bad
state:
  fields:
    - name: task
      type: string
nodes:
  - id: review
    type: agent
routing:
  - from: START
    to: review
  - from: review
    condition: 'workflow_status == "done"'
    to: END
    else: END
execution:
  maxSteps: 4
`)

	if result.Valid {
		t.Fatalf("workflow should be invalid")
	}
	if !containsWorkflowValidationError(result.Errors, "undefined state field: workflow_status") {
		t.Fatalf("errors=%v, want undefined workflow_status", result.Errors)
	}
}

func TestValidateSkillWorkflowYAMLRejectsLoopWithoutBudget(t *testing.T) {
	result := validateSkillWorkflowYAML(`
meta:
  name: bad-loop
state:
  fields:
    - name: task
      type: string
    - name: approved
      type: boolean
nodes:
  - id: author
    type: agent
  - id: review
    type: agent
routing:
  - from: START
    to: author
  - from: author
    to: review
  - from: review
    condition: 'approved != true'
    to: author
    else: END
`)

	if result.Valid {
		t.Fatalf("workflow should be invalid")
	}
	if !containsWorkflowValidationError(result.Errors, "requires execution.maxSteps") {
		t.Fatalf("errors=%v, want maxSteps error", result.Errors)
	}
}

func containsWorkflowValidationError(errors []string, want string) bool {
	for _, err := range errors {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
