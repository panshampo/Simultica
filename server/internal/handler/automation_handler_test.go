package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func createAutomationTestAgent(t *testing.T, name string) string {
	t.Helper()
	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id,
			instructions, custom_env, custom_args, mcp_config
		)
		VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'private', 1, $4, '', '{}'::jsonb, '[]'::jsonb, '{}'::jsonb)
		RETURNING id
	`, testWorkspaceID, name, testRuntimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})
	return agentID
}

func createAutomationTestTemplate(t *testing.T, agentID, title string) string {
	t.Helper()
	var templateID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO issue_template (
			workspace_id, title, issue_title_template, issue_body_template,
			assignee_type, assignee_id, priority, created_by_type, created_by_id
		)
		VALUES ($1, $2, $3, 'template body', 'agent', $4, 'high', 'member', $5)
		RETURNING id
	`, testWorkspaceID, title+" template", title, agentID, testUserID).Scan(&templateID); err != nil {
		t.Fatalf("create issue template: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue_template WHERE id = $1`, templateID)
	})
	return templateID
}

func TestSyncAutomationRunFromTask_CancelledMarksRunCancelled(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	agentID := createAutomationTestAgent(t, "automation-cancelled-sync-agent")

	var automationID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO automation (
			workspace_id, title, source_mode, inline_issue_config,
			status, concurrency_policy, created_by_type, created_by_id
		)
		VALUES (
			$1, 'cancelled sync automation', 'inline',
			'{"title":"Generated issue","description":"Automation description"}'::jsonb,
			'active', 'skip', 'member', $2
		)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&automationID); err != nil {
		t.Fatalf("setup: create automation: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM automation WHERE id = $1`, automationID) })

	var runID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO automation_run (
			automation_id, source, source_mode_snapshot, status,
			trigger_payload, resolved_issue_payload
		)
		VALUES (
			$1, 'manual', 'inline', 'running',
			'{"source":"test"}'::jsonb,
			'{"title":"Generated issue","description":"Automation description"}'::jsonb
		)
		RETURNING id
	`, automationID).Scan(&runID); err != nil {
		t.Fatalf("setup: create automation_run: %v", err)
	}

	bus := events.New()
	var donePayload map[string]any
	bus.Subscribe(protocol.EventAutopilotRunDone, func(e events.Event) {
		if payload, ok := e.Payload.(map[string]any); ok {
			donePayload = payload
		}
	})
	svc := service.NewAutopilotService(testHandler.Queries, testPool, bus, nil)

	svc.SyncAutomationRunFromTask(ctx, db.AgentTaskQueue{
		AgentID:         parseUUID(agentID),
		Status:          "cancelled",
		AutomationRunID: parseUUID(runID),
		Error:           pgtype.Text{String: "user cancelled task", Valid: true},
	})

	run, err := testHandler.Queries.GetAutomationRun(ctx, parseUUID(runID))
	if err != nil {
		t.Fatalf("get automation run: %v", err)
	}
	if run.Status != "cancelled" {
		t.Fatalf("automation_run.status = %q, want cancelled", run.Status)
	}
	if !run.CompletedAt.Valid {
		t.Fatal("automation_run.completed_at was not set")
	}
	if !run.FailureReason.Valid || run.FailureReason.String != "user cancelled task" {
		t.Fatalf("failure_reason = %#v, want user cancelled task", run.FailureReason)
	}
	if donePayload == nil {
		t.Fatal("expected automation run done event")
	}
	if donePayload["status"] != "cancelled" {
		t.Fatalf("done event status = %#v, want cancelled", donePayload["status"])
	}
}

func TestTriggerAutomation_TemplateModeCreatesIssueFromTemplate(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createAutomationTestAgent(t, "automation-template-agent")
	templateID := createAutomationTestTemplate(t, agentID, "Automation template issue")

	w := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api/automations?workspace_id="+testWorkspaceID, map[string]any{
		"title":              "template automation",
		"source_mode":        "template",
		"template_id":        templateID,
		"concurrency_policy": "skip",
	})
	testHandler.CreateAutomation(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAutomation: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var automation AutomationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &automation); err != nil {
		t.Fatalf("decode automation: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM automation_run WHERE automation_id = $1`, automation.ID)
		testPool.Exec(context.Background(), `DELETE FROM automation WHERE id = $1`, automation.ID)
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE automation_run_id IN (SELECT id FROM automation_run WHERE automation_id = $1)`, automation.ID)
	})

	w = httptest.NewRecorder()
	r = newRequest(http.MethodPost, "/api/automations/"+automation.ID+"/trigger?workspace_id="+testWorkspaceID, nil)
	r = withURLParam(r, "id", automation.ID)
	testHandler.TriggerAutomation(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("TriggerAutomation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var run AutomationRunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.Status != "issue_created" {
		t.Fatalf("run status = %q, want issue_created", run.Status)
	}
	if run.IssueID == nil {
		t.Fatal("run issue_id is nil")
	}

	dbRun, err := testHandler.Queries.GetAutomationRun(context.Background(), parseUUID(run.ID))
	if err != nil {
		t.Fatalf("get automation run: %v", err)
	}
	if len(dbRun.TemplateSnapshot) == 0 {
		t.Fatal("template mode run did not store template_snapshot")
	}
	if len(dbRun.ResolvedIssuePayload) == 0 {
		t.Fatal("template mode run did not store resolved_issue_payload")
	}
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(*run.IssueID))
	if err != nil {
		t.Fatalf("get created issue: %v", err)
	}
	if issue.Title != "Automation template issue" {
		t.Fatalf("issue title = %q, want Automation template issue", issue.Title)
	}
	if !issue.IssueTemplateID.Valid || uuidToString(issue.IssueTemplateID) != templateID {
		t.Fatalf("issue_template_id = %#v, want %s", issue.IssueTemplateID, templateID)
	}
	if len(issue.IssueTemplateSnapshot) == 0 {
		t.Fatal("created issue did not store issue_template_snapshot")
	}
}

func TestTriggerAutomation_InlineModeCreatesIssue(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createAutomationTestAgent(t, "automation-inline-agent")
	w := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api/automations?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "inline automation",
		"source_mode": "inline",
		"inline_issue_config": map[string]any{
			"issue_title_template": "Inline automation issue",
			"issue_body_template":  "inline body",
			"assignee_type":        "agent",
			"assignee_id":          agentID,
			"priority":             "medium",
		},
	})
	testHandler.CreateAutomation(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAutomation: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var automation AutomationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &automation); err != nil {
		t.Fatalf("decode automation: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM automation_run WHERE automation_id = $1`, automation.ID)
		testPool.Exec(context.Background(), `DELETE FROM automation WHERE id = $1`, automation.ID)
	})

	w = httptest.NewRecorder()
	r = newRequest(http.MethodPost, "/api/automations/"+automation.ID+"/trigger?workspace_id="+testWorkspaceID, nil)
	r = withURLParam(r, "id", automation.ID)
	testHandler.TriggerAutomation(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("TriggerAutomation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var run AutomationRunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.Status != "issue_created" {
		t.Fatalf("run status = %q, want issue_created", run.Status)
	}
	if run.IssueID == nil {
		t.Fatal("run issue_id is nil")
	}
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(*run.IssueID))
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if issue.Title != "Inline automation issue" {
		t.Fatalf("issue title = %q, want Inline automation issue", issue.Title)
	}
}

func TestTriggerAutomation_InlineModeRendersDateAndMetadata(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createAutomationTestAgent(t, "automation-inline-render-agent")
	w := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api/automations?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "inline automation",
		"source_mode": "inline",
		"inline_issue_config": map[string]any{
			"issue_title_template": "Inline automation issue {{date}}",
			"issue_body_template":  "inline body",
			"assignee_type":        "agent",
			"assignee_id":          agentID,
			"priority":             "medium",
		},
	})
	testHandler.CreateAutomation(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateAutomation: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var automation AutomationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &automation); err != nil {
		t.Fatalf("decode automation: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM automation WHERE id = $1`, automation.ID)
	})

	w = httptest.NewRecorder()
	r = newRequest(http.MethodPost, "/api/automations/"+automation.ID+"/trigger?workspace_id="+testWorkspaceID, nil)
	r = withURLParam(r, "id", automation.ID)
	testHandler.TriggerAutomation(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("TriggerAutomation: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var run AutomationRunResponse
	if err := json.Unmarshal(w.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.IssueID == nil {
		t.Fatal("run issue_id is nil")
	}
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(*run.IssueID))
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if strings.Contains(issue.Title, "{{date}}") {
		t.Fatalf("issue title did not interpolate date: %q", issue.Title)
	}
	if !strings.HasPrefix(issue.Title, "Inline automation issue 20") {
		t.Fatalf("issue title = %q, want rendered date suffix", issue.Title)
	}
	if !issue.Description.Valid || !strings.Contains(issue.Description.String, "inline body") || !strings.Contains(issue.Description.String, "Automation run triggered by manual") {
		t.Fatalf("issue description missing inline body or metadata: %q", issue.Description.String)
	}
}

func TestCreateAutomation_TemplateModeRejectsInlineFieldsAndRunOnly(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createAutomationTestAgent(t, "automation-validation-agent")
	templateID := createAutomationTestTemplate(t, agentID, "Validation issue")

	cases := []map[string]any{
		{
			"title":          "template with inline config",
			"source_mode":    "template",
			"template_id":    templateID,
			"execution_mode": "create_issue",
			"inline_issue_config": map[string]any{
				"issue_title_template": "invalid",
				"assignee_type":        "agent",
				"assignee_id":          agentID,
			},
		},
		{
			"title":          "template run only",
			"source_mode":    "template",
			"template_id":    templateID,
			"execution_mode": "run_only",
		},
	}
	for _, body := range cases {
		w := httptest.NewRecorder()
		r := newRequest(http.MethodPost, "/api/automations?workspace_id="+testWorkspaceID, body)
		testHandler.CreateAutomation(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("CreateAutomation(%v): expected 400, got %d: %s", body["title"], w.Code, w.Body.String())
		}
	}
}

func TestAutomationWebhook_DispatchesRun(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createAutomationTestAgent(t, "automation-webhook-agent")
	automation, err := testHandler.Queries.CreateAutomation(context.Background(), db.CreateAutomationParams{
		WorkspaceID:       parseUUID(testWorkspaceID),
		Title:             "webhook automation",
		SourceMode:        "inline",
		InlineIssueConfig: []byte(`{"issue_title_template":"Webhook automation issue {{date}}","issue_body_template":"webhook body","assignee_type":"agent","assignee_id":"` + agentID + `","priority":"medium"}`),
		Status:            "active",
		ConcurrencyPolicy: "skip",
		CreatedByType:     "member",
		CreatedByID:       parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create automation: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM automation_run WHERE automation_id = $1`, uuidToString(automation.ID))
		testPool.Exec(context.Background(), `DELETE FROM automation WHERE id = $1`, uuidToString(automation.ID))
	})
	trigger, err := testHandler.Queries.CreateAutomationTrigger(context.Background(), db.CreateAutomationTriggerParams{
		AutomationID: automation.ID,
		Kind:         "webhook",
		Enabled:      true,
		WebhookToken: ptrToText(stringPtr("awt_test_automation_webhook")),
		Provider:     ptrToText(stringPtr("generic")),
	})
	if err != nil {
		t.Fatalf("create automation trigger: %v", err)
	}

	w := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api/webhooks/automations/"+trigger.WebhookToken.String, map[string]any{"event": "automation.test"})
	r = withURLParam(r, "token", trigger.WebhookToken.String)
	testHandler.HandleAutomationWebhook(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HandleAutomationWebhook: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "accepted" {
		t.Fatalf("status = %#v, want accepted; body=%s", resp["status"], w.Body.String())
	}
	if resp["run_id"] == nil {
		t.Fatalf("missing run_id: %#v", resp)
	}
	if resp["delivery_id"] == nil {
		t.Fatalf("missing delivery_id: %#v", resp)
	}
	deliveryID := resp["delivery_id"].(string)

	w = httptest.NewRecorder()
	r = newRequest(http.MethodGet, "/api/automations/"+uuidToString(automation.ID)+"/deliveries?workspace_id="+testWorkspaceID, nil)
	r = withURLParam(r, "id", uuidToString(automation.ID))
	testHandler.ListAutomationDeliveries(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAutomationDeliveries: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Deliveries []WebhookDeliveryResponse `json:"deliveries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode deliveries: %v", err)
	}
	if len(listResp.Deliveries) != 1 {
		t.Fatalf("expected 1 automation delivery, got %d", len(listResp.Deliveries))
	}
	if listResp.Deliveries[0].AutomationRunID == nil {
		t.Fatalf("automation delivery did not link automation_run_id: %#v", listResp.Deliveries[0])
	}

	w = httptest.NewRecorder()
	r = newRequest(http.MethodGet, "/api/automations/"+uuidToString(automation.ID)+"/deliveries/"+deliveryID+"?workspace_id="+testWorkspaceID, nil)
	r = withURLParams(r, "id", uuidToString(automation.ID), "deliveryId", deliveryID)
	testHandler.GetAutomationDelivery(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GetAutomationDelivery: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var detail WebhookDeliveryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode delivery detail: %v", err)
	}
	if detail.RawBody == nil || !strings.Contains(*detail.RawBody, "automation.test") {
		t.Fatalf("detail did not include raw body: %#v", detail.RawBody)
	}

	run, err := testHandler.Queries.GetAutomationRun(context.Background(), parseUUID(resp["run_id"].(string)))
	if err != nil {
		t.Fatalf("get automation run: %v", err)
	}
	if !run.IssueID.Valid {
		t.Fatal("webhook run did not create issue")
	}
	issue, err := testHandler.Queries.GetIssue(context.Background(), run.IssueID)
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if strings.Contains(issue.Title, "{{date}}") {
		t.Fatalf("webhook issue title did not interpolate date: %q", issue.Title)
	}
	if !issue.Description.Valid || !strings.Contains(issue.Description.String, "webhook body") || !strings.Contains(issue.Description.String, "Webhook event: automation.test") {
		t.Fatalf("webhook issue description missing metadata/payload: %q", issue.Description.String)
	}

	w = httptest.NewRecorder()
	r = newRequest(http.MethodPost, "/api/webhooks/automations/"+trigger.WebhookToken.String, map[string]any{"event": "automation.test"})
	r.Header.Set("Idempotency-Key", "automation-delivery-dedupe")
	r = withURLParam(r, "token", trigger.WebhookToken.String)
	testHandler.HandleAutomationWebhook(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HandleAutomationWebhook first dedupe: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r = newRequest(http.MethodPost, "/api/webhooks/automations/"+trigger.WebhookToken.String, map[string]any{"event": "automation.test"})
	r.Header.Set("Idempotency-Key", "automation-delivery-dedupe")
	r = withURLParam(r, "token", trigger.WebhookToken.String)
	testHandler.HandleAutomationWebhook(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HandleAutomationWebhook duplicate: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var dupResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &dupResp); err != nil {
		t.Fatalf("decode duplicate response: %v", err)
	}
	if dupResp["status"] != "duplicate" {
		t.Fatalf("expected duplicate response, got %#v", dupResp)
	}
}

func TestAutomationWebhookReplay_CreatesNewDeliveryAndRun(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}
	agentID := createAutomationTestAgent(t, "automation-webhook-replay-agent")
	automation, err := testHandler.Queries.CreateAutomation(context.Background(), db.CreateAutomationParams{
		WorkspaceID:       parseUUID(testWorkspaceID),
		Title:             "webhook replay automation",
		SourceMode:        "inline",
		InlineIssueConfig: []byte(`{"issue_title_template":"Webhook replay issue {{date}}","issue_body_template":"webhook replay body","assignee_type":"agent","assignee_id":"` + agentID + `","priority":"medium"}`),
		Status:            "active",
		ConcurrencyPolicy: "skip",
		CreatedByType:     "member",
		CreatedByID:       parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create automation: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM issue WHERE automation_run_id IN (SELECT id FROM automation_run WHERE automation_id = $1)`, uuidToString(automation.ID))
		testPool.Exec(context.Background(), `DELETE FROM webhook_delivery WHERE automation_id = $1`, uuidToString(automation.ID))
		testPool.Exec(context.Background(), `DELETE FROM automation_run WHERE automation_id = $1`, uuidToString(automation.ID))
		testPool.Exec(context.Background(), `DELETE FROM automation WHERE id = $1`, uuidToString(automation.ID))
	})
	trigger, err := testHandler.Queries.CreateAutomationTrigger(context.Background(), db.CreateAutomationTriggerParams{
		AutomationID: automation.ID,
		Kind:         "webhook",
		Enabled:      true,
		WebhookToken: ptrToText(stringPtr("awt_test_automation_webhook_replay")),
		Provider:     ptrToText(stringPtr("generic")),
	})
	if err != nil {
		t.Fatalf("create automation trigger: %v", err)
	}

	w := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api/webhooks/automations/"+trigger.WebhookToken.String, map[string]any{"event": "automation.replay"})
	r.Header.Set("Idempotency-Key", "automation-replay-original")
	r = withURLParam(r, "token", trigger.WebhookToken.String)
	testHandler.HandleAutomationWebhook(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HandleAutomationWebhook: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var original map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &original); err != nil {
		t.Fatalf("decode original response: %v", err)
	}
	originalDeliveryID, _ := original["delivery_id"].(string)
	originalRunID, _ := original["run_id"].(string)
	if originalDeliveryID == "" || originalRunID == "" {
		t.Fatalf("original response missing delivery_id/run_id: %#v", original)
	}

	w = httptest.NewRecorder()
	r = newRequest(http.MethodPost, "/api/automations/"+uuidToString(automation.ID)+"/deliveries/"+originalDeliveryID+"/replay?workspace_id="+testWorkspaceID, nil)
	r = withURLParams(r, "id", uuidToString(automation.ID), "deliveryId", originalDeliveryID)
	testHandler.ReplayAutomationDelivery(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("ReplayAutomationDelivery: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var replay WebhookDeliveryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &replay); err != nil {
		t.Fatalf("decode replay response: %v", err)
	}
	if replay.ID == originalDeliveryID {
		t.Fatal("replay should create a new delivery")
	}
	if replay.AutomationRunID == nil || *replay.AutomationRunID == originalRunID {
		t.Fatalf("replay should create a new automation run, got %#v original=%s", replay.AutomationRunID, originalRunID)
	}
	if replay.AutomationID == nil || *replay.AutomationID != uuidToString(automation.ID) {
		t.Fatalf("replay automation_id = %#v, want %s", replay.AutomationID, uuidToString(automation.ID))
	}
	if replay.AutomationTriggerID == nil || *replay.AutomationTriggerID != uuidToString(trigger.ID) {
		t.Fatalf("replay automation_trigger_id = %#v, want %s", replay.AutomationTriggerID, uuidToString(trigger.ID))
	}
	if replay.ReplayedFromDeliveryID == nil || *replay.ReplayedFromDeliveryID != originalDeliveryID {
		t.Fatalf("replayed_from_delivery_id = %#v, want %s", replay.ReplayedFromDeliveryID, originalDeliveryID)
	}
	if replay.DedupeKey != nil {
		t.Fatalf("replay delivery should bypass dedupe with nil key, got %q", *replay.DedupeKey)
	}
}

func stringPtr(v string) *string { return &v }
