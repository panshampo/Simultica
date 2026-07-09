package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateGlobalSkillLinkCreatesWorkspaceSkillFromAllowedRoot(t *testing.T) {
	root, skillDir := createGlobalLinkFixture(t)
	t.Setenv("MULTICA_GLOBAL_SKILL_ROOTS", root)

	req := newRequest(http.MethodPost, "/api/skills/global-links?workspace_id="+testWorkspaceID, map[string]any{
		"target_path": skillDir,
	})
	w := httptest.NewRecorder()

	testHandler.CreateGlobalSkillLink(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("CreateGlobalSkillLink: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp SkillWithFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM skill WHERE id = $1`, resp.ID)
	})
	if resp.Name != "linked-skill" {
		t.Fatalf("name = %q, want linked-skill", resp.Name)
	}
	config := resp.Config.(map[string]any)
	if config["has_workflow"] != true {
		t.Fatalf("has_workflow = %#v, want true", config["has_workflow"])
	}
	origin := config["origin"].(map[string]any)
	if origin["type"] != originTypeGlobalLink {
		t.Fatalf("origin.type = %#v, want global_link", origin["type"])
	}
	if len(resp.Files) != 1 || resp.Files[0].Path != workflowFilePath {
		t.Fatalf("files = %+v, want only workflow.yaml", resp.Files)
	}
}

func TestValidateGlobalLinkWorkflowReadsResolvedBundle(t *testing.T) {
	root, skillDir := createGlobalLinkFixture(t)
	t.Setenv("MULTICA_GLOBAL_SKILL_ROOTS", root)
	skillID := createGlobalLinkSkillForTest(t, skillDir)

	req := newRequest(http.MethodPost, "/api/skills/"+skillID+"/workflow/validate", nil)
	req = withURLParam(req, "id", skillID)
	w := httptest.NewRecorder()

	testHandler.ValidateSkillWorkflow(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ValidateSkillWorkflow: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp SkillWorkflowValidationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Health.Status != "ok" || !resp.Health.HasWorkflow {
		t.Fatalf("health = %+v, want ok with workflow", resp.Health)
	}
	validation, ok := resp.Validation.(map[string]any)
	if !ok || validation["valid"] != true {
		t.Fatalf("validation = %#v, want valid", resp.Validation)
	}
}

func TestRefreshGlobalLinkReportsBrokenTarget(t *testing.T) {
	root, skillDir := createGlobalLinkFixture(t)
	t.Setenv("MULTICA_GLOBAL_SKILL_ROOTS", root)
	skillID := createGlobalLinkSkillForTest(t, skillDir)
	if err := os.RemoveAll(skillDir); err != nil {
		t.Fatalf("remove fixture skill: %v", err)
	}

	req := newRequest(http.MethodPost, "/api/skills/"+skillID+"/refresh", nil)
	req = withURLParam(req, "id", skillID)
	w := httptest.NewRecorder()

	testHandler.RefreshSkill(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("RefreshSkill: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ResolvedSkillBundleResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Health.Status != "broken" || len(resp.Health.Reasons) == 0 {
		t.Fatalf("health = %+v, want broken with reasons", resp.Health)
	}
}

func TestManualSkillUsesWorkspaceSymlinkOverlayInListAndDetail(t *testing.T) {
	root, skillDir := createGlobalLinkFixture(t)
	t.Setenv("MULTICA_GLOBAL_SKILL_ROOTS", root)
	name := "linked-skill-manual-overlay"
	restore := installWorkspaceSkillSymlinkForTest(t, name, skillDir)
	defer restore()
	skillID := insertHandlerTestSkillExact(t, name, "# stale db copy", "{}")

	listReq := newRequest(http.MethodGet, "/api/skills?workspace_id="+testWorkspaceID, nil)
	listW := httptest.NewRecorder()
	testHandler.ListSkills(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListSkills: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var rows []SkillSummaryResponse
	if err := json.NewDecoder(listW.Body).Decode(&rows); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	var found *SkillSummaryResponse
	for i := range rows {
		if rows[i].ID == skillID {
			found = &rows[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("linked-skill row not found")
	}
	config := found.Config.(map[string]any)
	origin := config["origin"].(map[string]any)
	if origin["type"] != originTypeGlobalLink {
		t.Fatalf("list origin.type = %#v, want global_link", origin["type"])
	}

	detailReq := newRequest(http.MethodGet, "/api/skills/"+skillID, nil)
	detailReq = withURLParam(detailReq, "id", skillID)
	detailW := httptest.NewRecorder()
	testHandler.GetSkill(detailW, detailReq)
	if detailW.Code != http.StatusOK {
		t.Fatalf("GetSkill: expected 200, got %d: %s", detailW.Code, detailW.Body.String())
	}
	var detail SkillWithFilesResponse
	if err := json.NewDecoder(detailW.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	if detail.Content == "# stale db copy" {
		t.Fatalf("detail content used stale DB copy, want symlink target content")
	}
	if len(detail.Files) != 1 || detail.Files[0].Path != workflowFilePath {
		t.Fatalf("detail files = %+v, want symlink workflow file", detail.Files)
	}
}

func TestWorkspaceSymlinkOverlayWinsOverImportedOrigin(t *testing.T) {
	root, skillDir := createGlobalLinkFixture(t)
	t.Setenv("MULTICA_GLOBAL_SKILL_ROOTS", root)
	name := "linked-skill-import-overlay"
	restore := installWorkspaceSkillSymlinkForTest(t, name, skillDir)
	defer restore()
	skillID := insertHandlerTestSkillExact(t, name, "# imported db copy", `{"origin":{"type":"github","source_url":"https://example.test/old"}}`)

	req := newRequest(http.MethodGet, "/api/skills?workspace_id="+testWorkspaceID, nil)
	w := httptest.NewRecorder()
	testHandler.ListSkills(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListSkills: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var rows []SkillSummaryResponse
	if err := json.NewDecoder(w.Body).Decode(&rows); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	for _, row := range rows {
		if row.ID != skillID {
			continue
		}
		config := row.Config.(map[string]any)
		origin := config["origin"].(map[string]any)
		if origin["type"] != originTypeGlobalLink {
			t.Fatalf("origin.type = %#v, want global_link", origin["type"])
		}
		return
	}
	t.Fatalf("linked-skill row not found")
}

func createGlobalLinkFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	skillDir := filepath.Join(root, "linked-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: linked-skill
description: Linked skill fixture.
---

# Linked Skill
`), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, workflowFilePath), []byte(validWorkflowYAML()), 0o644); err != nil {
		t.Fatalf("write workflow.yaml: %v", err)
	}
	return root, skillDir
}

func createGlobalLinkSkillForTest(t *testing.T, skillDir string) string {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/skills/global-links?workspace_id="+testWorkspaceID, map[string]any{
		"target_path": skillDir,
	})
	w := httptest.NewRecorder()
	testHandler.CreateGlobalSkillLink(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateGlobalSkillLink: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp SkillWithFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM skill WHERE id = $1`, resp.ID)
	})
	return resp.ID
}

func installWorkspaceSkillSymlinkForTest(t *testing.T, name, target string) func() {
	t.Helper()
	linkPath, err := workspaceSkillLinkPath(name)
	if err != nil {
		t.Fatalf("workspace skill link path: %v", err)
	}
	parent := filepath.Dir(linkPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("mkdir workspace skill root: %v", err)
	}
	backup := linkPath + ".bak-test"
	hadExisting := false
	if _, err := os.Lstat(linkPath); err == nil {
		hadExisting = true
		if err := os.Rename(linkPath, backup); err != nil {
			t.Fatalf("backup existing skill link: %v", err)
		}
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("create workspace skill symlink: %v", err)
	}
	return func() {
		_ = os.Remove(linkPath)
		if hadExisting {
			_ = os.Rename(backup, linkPath)
		}
	}
}

func insertHandlerTestSkillExact(t *testing.T, name, content, config string) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO skill (workspace_id, name, description, content, config, created_by)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
		RETURNING id
	`, testWorkspaceID, name, "fixture", content, config, testUserID).Scan(&id); err != nil {
		t.Fatalf("insert skill: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM skill WHERE id = $1`, id)
	})
	return id
}
