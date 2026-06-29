package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpsertWorkflowFileSetsHasWorkflowFlag(t *testing.T) {
	skillID := insertHandlerTestSkill(t, "workflow-upsert", "# skill")

	req := newRequest("PUT", "/api/skills/"+skillID+"/files", map[string]any{
		"path":    workflowFilePath,
		"content": validWorkflowYAML(),
	})
	req = withURLParam(req, "id", skillID)
	w := httptest.NewRecorder()

	testHandler.UpsertSkillFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpsertSkillFile: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertSkillHasWorkflow(t, skillID, true)
	assertSkillWorkflowValidation(t, skillID, true)
}

func TestUpsertWorkflowFileStoresValidationErrors(t *testing.T) {
	skillID := insertHandlerTestSkill(t, "workflow-invalid", "# skill")

	req := newRequest("PUT", "/api/skills/"+skillID+"/files", map[string]any{
		"path": workflowFilePath,
		"content": `
meta:
  name: invalid
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
`,
	})
	req = withURLParam(req, "id", skillID)
	w := httptest.NewRecorder()

	testHandler.UpsertSkillFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpsertSkillFile: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertSkillHasWorkflow(t, skillID, true)
	assertSkillWorkflowValidation(t, skillID, false)
}

func TestDeleteWorkflowFileClearsHasWorkflowFlag(t *testing.T) {
	ctx := context.Background()
	skillID := insertHandlerTestSkill(t, "workflow-delete", "# skill")
	var fileID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO skill_file (skill_id, path, content)
		VALUES ($1, $2, 'meta: {}')
		RETURNING id
	`, skillID, workflowFilePath).Scan(&fileID); err != nil {
		t.Fatalf("seed workflow file: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE skill SET config='{"has_workflow": true}'::jsonb WHERE id=$1
	`, skillID); err != nil {
		t.Fatalf("seed workflow flag: %v", err)
	}

	req := newRequest("DELETE", "/api/skills/"+skillID+"/files/"+fileID, nil)
	req = withURLParams(req, "id", skillID, "fileId", fileID)
	w := httptest.NewRecorder()

	testHandler.DeleteSkillFile(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteSkillFile: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	assertSkillHasWorkflow(t, skillID, false)
}

func TestUpdateSkillFilesSetsHasWorkflowFlagFromReplacementList(t *testing.T) {
	skillID := insertHandlerTestSkill(t, "workflow-replace", "# skill")

	req := newRequest("PUT", "/api/skills/"+skillID, map[string]any{
		"files": []map[string]any{
			{"path": "notes.md", "content": "notes"},
			{"path": workflowFilePath, "content": validWorkflowYAML()},
		},
	})
	req = withURLParam(req, "id", skillID)
	w := httptest.NewRecorder()

	testHandler.UpdateSkill(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpdateSkill: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertSkillHasWorkflow(t, skillID, true)
	assertSkillWorkflowValidation(t, skillID, true)

	req = newRequest("PUT", "/api/skills/"+skillID, map[string]any{
		"files": []map[string]any{
			{"path": "notes.md", "content": "notes"},
		},
	})
	req = withURLParam(req, "id", skillID)
	w = httptest.NewRecorder()

	testHandler.UpdateSkill(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("UpdateSkill clear: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertSkillHasWorkflow(t, skillID, false)
}

func validWorkflowYAML() string {
	return `
meta:
  name: wf
  version: "1"
state:
  fields:
    - name: task
      type: string
    - name: workflow_status
      type: string
    - name: revisionCount
      type: number
nodes:
  - id: author
    type: agent
  - id: review
    type: agent
    on_complete:
      - action: increment
        field: revisionCount
routing:
  - from: START
    to: author
  - from: author
    to: review
  - from: review
    condition: 'workflow_status == "fixable_auto" && revisionCount < 3'
    to: author
    else: END
execution:
  maxSteps: 8
`
}

func assertSkillHasWorkflow(t *testing.T, skillID string, want bool) {
	t.Helper()
	var raw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT config FROM skill WHERE id=$1`, skillID).Scan(&raw); err != nil {
		t.Fatalf("read skill config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode skill config %s: %v", string(raw), err)
	}
	if got, _ := cfg["has_workflow"].(bool); got != want {
		t.Fatalf("has_workflow = %v, want %v (config=%s)", got, want, string(raw))
	}
}

func assertSkillWorkflowValidation(t *testing.T, skillID string, wantValid bool) {
	t.Helper()
	var raw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT config FROM skill WHERE id=$1`, skillID).Scan(&raw); err != nil {
		t.Fatalf("read skill config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode skill config %s: %v", string(raw), err)
	}
	validation, ok := cfg["workflow_validation"].(map[string]any)
	if !ok {
		t.Fatalf("workflow_validation missing in config=%s", string(raw))
	}
	if got, _ := validation["valid"].(bool); got != wantValid {
		t.Fatalf("workflow_validation.valid = %v, want %v (config=%s)", got, wantValid, string(raw))
	}
	if !wantValid {
		errors, _ := validation["errors"].([]any)
		if len(errors) == 0 {
			t.Fatalf("workflow_validation errors missing for invalid workflow: %s", string(raw))
		}
	}
}
