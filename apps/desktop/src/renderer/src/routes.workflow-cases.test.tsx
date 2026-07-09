import { describe, expect, it, vi } from "vitest";
import { matchRoutes } from "react-router-dom";

vi.mock("@multica/views/workflow", () => ({
  WorkflowCaseListPage: () => null,
  WorkflowCaseDetailPage: () => null,
}));

vi.mock("@multica/views/issues/components", () => ({
  IssuesPage: () => null,
  IssueDetail: () => null,
}));

vi.mock("@multica/views/projects/components", () => ({
  ProjectsPage: () => null,
  ProjectDetail: () => null,
}));

vi.mock("@multica/views/templates", () => ({
  TemplatesPage: () => null,
  TemplateDetailPage: () => null,
}));

vi.mock("@multica/views/dashboard", () => ({
  DashboardPage: () => null,
}));

vi.mock("@multica/views/autopilots/components", () => ({
  AutopilotsPage: () => null,
  AutopilotDetailPage: () => null,
}));

vi.mock("@multica/views/my-issues", () => ({
  MyIssuesPage: () => null,
}));

vi.mock("@multica/views/skills", () => ({
  SkillsPage: () => null,
  SkillDetailPage: () => null,
}));

vi.mock("@multica/views/squads/components", () => ({
  SquadsPage: () => null,
  SquadDetailPage: () => null,
}));

vi.mock("@multica/views/inbox", () => ({
  InboxPage: () => null,
}));

vi.mock("@multica/views/settings", () => ({
  SettingsPage: () => null,
}));

vi.mock("@multica/views/i18n", () => ({
  useT: () => ({ t: (fn: (dict: Record<string, unknown>) => unknown) => String(fn({ desktop: { tabs: { server: "Server", updates: "Updates" } } })) }),
}));

vi.mock("./pages/issue-detail-page", () => ({ IssueDetailPage: () => null }));
vi.mock("./pages/project-detail-page", () => ({ ProjectDetailPage: () => null }));
vi.mock("./pages/autopilot-detail-page", () => ({ AutopilotDetailPage: () => null }));
vi.mock("./pages/skill-detail-page", () => ({ SkillDetailPage: () => null }));
vi.mock("./pages/agent-detail-page", () => ({ AgentDetailPage: () => null }));
vi.mock("./pages/member-detail-page", () => ({ MemberDetailPage: () => null }));
vi.mock("./pages/runtime-detail-page", () => ({ RuntimeDetailPage: () => null }));
vi.mock("./pages/attachment-preview-page", () => ({ AttachmentPreviewRoute: () => null }));
vi.mock("./components/desktop-runtimes-page", () => ({ DesktopRuntimesPage: () => null }));
vi.mock("./components/desktop-agents-page", () => ({ DesktopAgentsPage: () => null }));
vi.mock("./components/daemon-settings-tab", () => ({ DaemonSettingsTab: () => null }));
vi.mock("./components/server-settings-tab", () => ({ ServerSettingsTab: () => null }));
vi.mock("./components/updates-settings-tab", () => ({ UpdatesSettingsTab: () => null }));
vi.mock("./components/workspace-route-layout", () => ({ WorkspaceRouteLayout: () => null }));
vi.mock("./components/route-error-page", () => ({ DesktopRouteErrorPage: () => null }));

import { appRoutes } from "./routes";

describe("desktop WorkflowCase routes", () => {
  it("matches WorkflowCase list and detail tab paths", () => {
    expect(matchRoutes(appRoutes, "/acme/workflow-cases")).not.toBeNull();
    expect(matchRoutes(appRoutes, "/acme/workflow-cases/case-1")).not.toBeNull();
    expect(matchRoutes(appRoutes, "/acme/workflow-cases/case-1/versions/version-1")).not.toBeNull();
  });
});
