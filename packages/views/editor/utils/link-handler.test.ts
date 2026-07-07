import { describe, expect, it, vi } from "vitest";

import { openLink } from "./link-handler";

function expectNavigatePath(
  href: string,
  currentSlug: string | null | undefined,
  expectedPath: string,
) {
  const listener = vi.fn();
  window.addEventListener("multica:navigate", listener);

  openLink(href, currentSlug);

  expect(listener).toHaveBeenCalledOnce();
  const event = listener.mock.calls[0]?.[0] as CustomEvent<{ path: string }>;
  expect(event.detail.path).toBe(expectedPath);
  window.removeEventListener("multica:navigate", listener);
}

describe("openLink workspace route normalization", () => {
  it("prepends the current workspace slug to legacy autopilot paths", () => {
    expectNavigatePath("/autopilots/a1", "acme", "/acme/autopilots/a1");
  });

  it("prepends the current workspace slug to workflow case paths", () => {
    expectNavigatePath("/workflow-cases/c1", "acme", "/acme/workflow-cases/c1");
  });

  it("leaves already-slugged autopilot paths unchanged", () => {
    expectNavigatePath("/acme/autopilots/a1", "acme", "/acme/autopilots/a1");
  });

  it("leaves already-slugged workflow case paths unchanged", () => {
    expectNavigatePath("/acme/workflow-cases/c1", "acme", "/acme/workflow-cases/c1");
  });
});
