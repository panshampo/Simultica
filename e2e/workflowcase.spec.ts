import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("WorkflowCase", () => {
  let api: TestApiClient | undefined;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api?.cleanup();
    api = undefined;
  });

  test("can create, confirm, run, and inspect a WorkflowCase from the case entry", async ({ page }) => {
    const stamp = Date.now();
    const workflowCase = await api.createWorkflowCase({
      title: `E2E WorkflowCase ${stamp}`,
      description: "WorkflowCase first end-to-end smoke",
    });

    await page.goto(`/e2e-workspace/workflow-cases/${workflowCase.id}`);

    await expect(page.getByRole("heading", { name: `E2E WorkflowCase ${stamp}` })).toBeVisible({ timeout: 15000 });
    await expect(page.getByText("No definition draft is available for this workflow case.")).toBeVisible();
    await page.getByRole("button", { name: "Create starter draft" }).last().click();
    await expect(page.getByLabel("Workflow YAML")).toBeVisible();
    await page.getByRole("button", { name: "Apply YAML" }).click();
    await page.getByRole("button", { name: "Save draft" }).click();
    await expect(page.getByText("Draft saved")).toBeVisible({ timeout: 10000 });

    await expect(page.getByText("Definition draft")).toBeVisible();
    await expect(page.getByText("Validation has not been run for this draft.")).toBeVisible();

    const confirm = page.getByRole("button", { name: "Confirm and run" });
    await expect(confirm).toBeDisabled();

    await page.getByRole("button", { name: "Validate" }).click();
    await expect(page.getByText("Validation passed")).toBeVisible({ timeout: 10000 });
    await expect(confirm).toBeEnabled();

    await confirm.click();

    await expect.poll(async () => {
      const body = await api.getWorkflowCaseRuns(workflowCase.id);
      return body.runs?.[0]?.nodes?.length ?? 0;
    }, { timeout: 15000 }).toBeGreaterThan(0);

    await page.reload();
    await expect(page.getByText("Run history")).toBeVisible({ timeout: 15000 });
    await dismissOptionalQuickQuestion(page);
    await page.locator(".react-flow__node", { hasText: "gate" }).last().click();
    await expect(page.getByText("Node detail")).toBeVisible({ timeout: 10000 });
    const detailPanel = page.getByRole("complementary");
    await expect(detailPanel.getByText("gate")).toBeVisible();
    await expect(detailPanel.getByText("inline")).toBeVisible();
  });
});

async function dismissOptionalQuickQuestion(page: import("@playwright/test").Page) {
  const dialog = page.getByRole("dialog").filter({ hasText: "Quick question" });
  if (await dialog.isVisible().catch(() => false)) {
    await dialog.getByRole("button", { name: "Skip" }).click();
    await expect(dialog).toBeHidden({ timeout: 5000 });
  }
}
