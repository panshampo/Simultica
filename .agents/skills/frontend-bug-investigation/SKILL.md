---
name: frontend-bug-investigation
description: Use when investigating suspected frontend product bugs, unstable UI behavior, route/query loss, form state drift, request races, permission mismatches, browser runtime errors, or Nova nova-platform issues that need Playwright evidence.
metadata:
  author: local
  version: "1.0.0"
  argument-hint: <frontend bug, page, route, module, or suspicion>
---

# Frontend Bug Investigation

## Purpose

Find and validate frontend bugs without leaving debug code behind. This skill is for investigation, evidence, repro, likely root cause, and test recommendation. It is not a fix workflow.

This skill includes `workflow.yaml`. In Simultica, matching issue assignments should use the workflow instead of hand-creating ad hoc sub-issues.

## Hard Rules

- Do not finish with agent-created code changes still present.
- Do not revert or erase user pre-existing changes.
- Capture a baseline before debugging, then return the repository to that baseline before claiming completion.
- Temporary instrumentation is allowed only while investigating: logs, local probes, one-off Playwright scripts, debug assertions, mock toggles, or local test IDs.
- Remove all agent-created temporary tracked changes and untracked debug files before the final answer, unless the user explicitly asks to keep a specific artifact.
- If cleanup cannot return to baseline, stop and report the remaining delta. Do not say the investigation is complete.

## When to Use

Use for:

- frontend behavior that is broken, flaky, or hard to reproduce
- route, query, history, tab, or cache behavior that may lose context
- form state, async hydration, conditional fields, or submit payload drift
- UI permission, enabled/disabled, hidden/callable, or batch-action mismatches
- console errors, `pageerror`, failed requests, chunk load failures, blank or degraded screens
- code review that should produce verified frontend bug candidates instead of only suspicions

Do not use as the primary workflow when the issue is clearly backend-only, data-only, permission configuration-only, or when the user has asked to implement a fix immediately.

## Baseline Gate

At the start of the target repository:

```bash
git status --short
git diff --stat
git ls-files --others --exclude-standard
```

Record which files were already modified or untracked. Treat that as the baseline. The cleanup target is not a clean repository; it is the same tracked and untracked state that existed before this investigation began.

If the repository is not a git worktree, create an explicit temporary-artifact list and use that list for cleanup.

## Investigation Flow

1. Map entry points: page route, component boundary, state owner, request hooks, generated clients, cache keys, and relevant feature flags.
2. Search high-signal patterns: `TODO`, `FIXME`, `@ts-ignore`, `any`, `Boolean(`, `||`, `setFieldsValue`, `resetFields`, `useEffect`, `useMemo`, `localStorage`, `sessionStorage`, `history`, `navigate`, `push`, `goBack`, `location.search`, `permission`, `disabled`, `batch`, `dangerouslySetInnerHTML`.
3. Read surrounding code until the user action, expected state, request payload, response handling, render state, and error path are clear.
4. Compare sibling implementations, old/refactored modules, route builders, generated API clients, and tests.
5. Create bug candidates only when there is a concrete validation path.
6. Validate candidates with the smallest reliable runtime check: unit/component test, local browser probe, Playwright script, existing E2E case, or browser log capture.
7. Classify each candidate as `confirmed bug`, `high-risk suspicion`, `needs business confirmation`, or `blocked by env/data`.
8. Recommend automated coverage for confirmed bugs and high-risk suspicions.
9. Run the cleanup gate and verify the repository returned to baseline.

## Frontend Lenses

- State and effects: stale closures, missing dependencies, duplicate fetches, request races, unmounted updates, late hydration overwriting user edits.
- Forms: hidden required fields, disabled submitted fields, stale values after condition changes, `0` or `false` lost through `||`, normalizers, or truthy checks.
- Routing and cache: query loss, string boolean mistakes, unscoped cache keys, history stack assumptions, tab/source context loss.
- Async and network: swallowed request errors, optimistic UI not rolled back, stale response winning, retry duplicating side effects, loading state stuck.
- Permissions: button state and callable action diverge, batch operation checks only one row, read and write eligibility come from different sources.
- Rendering safety: unsafe HTML, dynamic labels rendered unsafely, missing empty/error states, runtime crash behind optional data.
- Duplicates: old/new/refactored components with diverged validation, payload, route, or permission rules.

## Validation Evidence

Strong evidence includes:

- browser `console.error` or `console.warn` tied to the repro
- uncaught `pageerror`
- `requestfailed`, 4xx/5xx, CORS, or chunk load failures
- final URL, route params, query string, and visible UI state
- request payload and response shape when safe to inspect
- screenshot, trace, or focused Playwright failure artifact
- isolated unit/component test that reproduces the state transition

Weak evidence includes:

- code smell without a reachable user path
- a failing assertion that depends on brittle selectors or unknown data
- behavior that needs product confirmation before being called wrong

## Nova Platform Profile

Use this profile for `/Users/bytedance/nova/nova-platform`.

Nova frontend characteristics:

- EdenX + React 17 application, with React Router/history v5 style behavior through the runtime stack.
- Garfish micro-frontend under the Pearl shell, so host history and sub-app history can diverge from ordinary SPA assumptions.
- Context parameters such as `env`, `oec_region`, `app_group`, tab/source query, and business IDs are part of the behavior contract.
- Arco Design, generated clients, Rematch/buildContext, route helpers, and conditional form flows are common bug surfaces.
- `apps/nova-e2e` is the preferred home for Playwright evidence and registered business-flow validation.

Nova is especially suitable for this skill when investigating:

- Back/Cancel/navigation failures, query loss, source detail/list return bugs, or Garfish history anomalies.
- Form hydration, step flow, conditional field residue, validation mismatch, or submit payload drift.
- Startup/runtime failures visible through console, pageerror, failed network responses, CORS, or chunk load errors.
- Region/env/app-group cache pollution or context loss across navigation.
- UI permission and batch-action state mismatches.

For Nova validation:

- Reuse an existing reachable nova-platform dev server before starting a new one.
- Prefer `apps/nova-e2e` Playwright infrastructure over ad hoc browser automation when a case maps to a known flow.
- For candidate triage, a short Playwright probe may capture `console`, `pageerror`, `requestfailed`, `response >= 400`, final URL, title, and screenshot.
- For registered E2E work, use the `nova-platform-e2e` skill and follow its case registry, mutation gate, selector, and real/PPE validation rules.
- Do not inspect cookies, localStorage, or saved profile secrets unless the user explicitly authorizes that scope.

Common Nova commands and locations:

```bash
cd /Users/bytedance/nova/nova-platform/apps/nova-e2e
pnpm run e2e:local -- <spec> --grep "<case title or tag>"
pnpm run e2e:real -- <spec> --grep "<case title or tag>"
node --test scripts/__tests__/case-registry.test.js
```

Use exact commands from the current repo after checking `package.json`, README, and local scripts; paths and ports drift.

## Workflow

Use `workflow.yaml` when this skill is bound to a Simultica issue. The workflow runs:

1. `scope_and_baseline`: identify target repo, scope, suspected surfaces, and baseline state.
2. `investigate_and_verify`: inspect code, create bug candidates, and gather runtime evidence.
3. `review_cleanup`: review evidence quality, enforce cleanup, and decide whether one more investigation pass is safe.
4. `final_report`: summarize confirmed bugs, unresolved suspicions, repro paths, likely root causes, test recommendations, and cleanup status.

The workflow may loop back from `review_cleanup` to `investigate_and_verify` when the result is fixable by more evidence gathering and `revisionCount < 2`.

## Cleanup Gate

Before the final response:

```bash
git status --short
git diff --stat
git ls-files --others --exclude-standard
```

Compare with the baseline. Remove only agent-created temporary artifacts. Restore only agent-created tracked edits; never revert user pre-existing changes.

The final answer must include:

- whether tracked state returned to baseline
- whether agent-created untracked debug artifacts were removed
- any remaining delta and why it remains

## Output Format

For each investigated case:

- `Bug candidate`: concise title
- `Confidence`: `confirmed bug`, `high-risk suspicion`, `needs business confirmation`, or `blocked by env/data`
- `Evidence`: file/line evidence and runtime evidence, if any
- `Why suspicious`: expected behavior vs observed or likely failure
- `Validation path`: what was run or what must be run
- `Minimal repro`: steps, route, data prerequisites, and expected visible result
- `Likely root cause`: narrow code-path explanation, not a broad guess
- `Recommended coverage`: unit, component, Playwright local, Playwright real, PPE, or manual product confirmation
- `Cleanup status`: state whether the repo returned to baseline

Order findings by user impact and confidence. Prefer fewer well-evidenced findings over many speculative ones.

## Common Mistakes

- Calling a code smell a bug before finding a reachable user path.
- Keeping debug `console.log`, temporary test IDs, temporary specs, or local probes after the investigation.
- Treating "clean git status" as the goal when the user had pre-existing changes; the goal is return to baseline.
- Weakening selectors or assertions instead of reading artifacts and frontend code.
- Running Nova PPE validation without the required parity checks from the E2E workflow.
- Treating a backend business error as selector flakiness.
