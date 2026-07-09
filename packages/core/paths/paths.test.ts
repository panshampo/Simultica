import { describe, it, expect } from "vitest";
import { paths, isGlobalPath } from "./paths";

describe("paths.workspace(slug)", () => {
  const ws = paths.workspace("acme");

  it("builds workspace paths with slug prefix", () => {
    expect(ws.usage()).toBe("/acme/usage");
    expect(ws.issues()).toBe("/acme/issues");
    expect(ws.issueDetail("abc-123")).toBe("/acme/issues/abc-123");
    expect(ws.projects()).toBe("/acme/projects");
    expect(ws.projectDetail("p1")).toBe("/acme/projects/p1");
    expect(ws.templates()).toBe("/acme/templates");
    expect(ws.templateDetail("tpl_1")).toBe("/acme/templates/tpl_1");
    expect(ws.workflowCases()).toBe("/acme/workflow-cases");
    expect(ws.workflowCaseDetail("case_1")).toBe("/acme/workflow-cases/case_1");
    expect(ws.workflowCaseVersionDetail("case_1", "version_1")).toBe(
      "/acme/workflow-cases/case_1/versions/version_1",
    );
    expect(ws.workflowCaseRunDetail("case_1", "run_1")).toBe(
      "/acme/workflow-cases/case_1/runs/run_1",
    );
    expect(ws.workflowCaseRunDetail("case_1", "run_1", "node_1")).toBe(
      "/acme/workflow-cases/case_1/runs/run_1?node_id=node_1",
    );
    expect(ws.workflowCaseRunNode("case_1", "run_1", "node_1")).toBe(
      "/acme/workflow-cases/case_1?run_id=run_1&node_id=node_1",
    );
    expect(ws.workflowCaseRunNode("case_1", "run_1")).toBe(
      "/acme/workflow-cases/case_1?run_id=run_1",
    );
    expect(ws.automations()).toBe("/acme/automations");
    expect(ws.automationDetail("auto_1")).toBe("/acme/automations/auto_1");
    expect(ws.autopilots()).toBe("/acme/autopilots");
    expect(ws.autopilotDetail("a1")).toBe("/acme/autopilots/a1");
    expect(ws.agents()).toBe("/acme/agents");
    expect(ws.memberDetail("u1")).toBe("/acme/members/u1");
    expect(ws.inbox()).toBe("/acme/inbox");
    expect(ws.myIssues()).toBe("/acme/my-issues");
    expect(ws.runtimes()).toBe("/acme/runtimes");
    expect(ws.skills()).toBe("/acme/skills");
    expect(ws.skillDetail("skl_123")).toBe("/acme/skills/skl_123");
    expect(ws.squads()).toBe("/acme/squads");
    expect(ws.squadDetail("sq_1")).toBe("/acme/squads/sq_1");
    expect(ws.settings()).toBe("/acme/settings");
    expect(ws.attachmentPreview("att_42")).toBe("/acme/attachments/att_42/preview");
  });

  it("URL-encodes special characters in ids", () => {
    expect(ws.issueDetail("id with space")).toBe("/acme/issues/id%20with%20space");
  });
});

describe("paths (global)", () => {
  it("builds global paths without slug", () => {
    expect(paths.login()).toBe("/login");
    expect(paths.newWorkspace()).toBe("/workspaces/new");
    expect(paths.invite("inv-1")).toBe("/invite/inv-1");
    expect(paths.authCallback()).toBe("/auth/callback");
  });
});

describe("isGlobalPath", () => {
  it("returns true for pre-workspace routes", () => {
    expect(isGlobalPath("/login")).toBe(true);
    expect(isGlobalPath("/workspaces/new")).toBe(true);
    expect(isGlobalPath("/invite/abc")).toBe(true);
    expect(isGlobalPath("/auth/callback")).toBe(true);
  });

  it("returns false for workspace-scoped paths", () => {
    expect(isGlobalPath("/acme/issues")).toBe(false);
    expect(isGlobalPath("/")).toBe(false);
  });
});
