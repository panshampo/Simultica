# Skill Content Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework the Skills Content tab into a compact content workbench with a file-type renderer registry, larger file body area, and overflow-safe skill description.

**Architecture:** Keep this UI-only and local to the Skills frontend. Extract file rendering into a local renderer registry consumed by `FileViewer`, then simplify `SkillDetailPage` so metadata is folded into a compact header and the selected file workspace owns the rest of the main panel.

**Tech Stack:** React 17-style function components, TypeScript, Tailwind classes, existing `../../common/markdown`, Vitest + Testing Library, Node 22 for verification.

## Global Constraints

- Do not change server APIs, skill storage, database schema, workflow runtime, agent runtime, or global Markdown behavior.
- Do not introduce split view; keep single-pane Preview/Edit switching.
- Do not add Mermaid, document outline, reading progress, or a new Markdown parser.
- Keep file renderer registry local to the Skills module.
- Use existing UI components and styles where possible.

---

### Task 1: Add Local Skill File Renderer Registry

**Files:**
- Create: `packages/views/skills/components/skill-file-renderers.tsx`
- Modify: `packages/views/skills/components/file-viewer.tsx`
- Test: `packages/views/skills/components/skill-file-renderers.test.tsx`

**Interfaces:**
- Produces:
  - `type SkillFileMode = "preview" | "edit"`
  - `type SkillFileRenderProps = { path: string; content: string; canEdit: boolean; onChange: (content: string) => void }`
  - `type SkillFileRenderer = { id: string; label: string; matches: (path: string) => boolean; defaultMode: SkillFileMode; canPreview: boolean; Preview?: ComponentType<SkillFileRenderProps>; Editor: ComponentType<SkillFileRenderProps> }`
  - `getSkillFileRenderer(path: string): SkillFileRenderer`
  - `isMarkdownFile(path: string): boolean`
- Consumes existing `parseFrontmatter`, `Textarea`, and `Markdown`.

- [ ] **Step 1: Write renderer registry tests**

Create tests that assert:

```ts
expect(getSkillFileRenderer("SKILL.md").id).toBe("markdown");
expect(getSkillFileRenderer("README.mdx").id).toBe("markdown");
expect(getSkillFileRenderer("workflow.yaml").id).toBe("yaml");
expect(getSkillFileRenderer("config.yml").id).toBe("yaml");
expect(getSkillFileRenderer("package.json").id).toBe("json");
expect(getSkillFileRenderer("notes.txt").id).toBe("text");
```

- [ ] **Step 2: Implement registry and renderers**

Create local renderers:

- Markdown preview: parse frontmatter, render compact frontmatter panel, then render body through existing `Markdown mode="full"` inside overflow-safe document styles.
- Markdown editor: textarea.
- YAML/JSON/text preview: preformatted overflow-safe text.
- YAML/JSON/text editor: textarea.

- [ ] **Step 3: Refactor `FileViewer`**

`FileViewer` should:

- Accept `canEdit?: boolean`.
- Resolve renderer by selected path.
- Keep single Preview/Edit toggle.
- Hide or disable edit mode when `canEdit` is false.
- Reset mode to renderer default when path changes.
- Render toolbar with path, renderer label, and Preview/Edit control.

- [ ] **Step 4: Run focused tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- skills/components/skill-file-renderers.test.tsx
```

Expected: tests pass.

---

### Task 2: Rework Skill Detail Content Layout

**Files:**
- Modify: `packages/views/skills/components/skill-detail-page.tsx`
- Modify: `packages/views/locales/en/skills.json`
- Modify: `packages/views/locales/zh-Hans/skills.json`

**Interfaces:**
- Consumes `FileViewer` with `canEdit`.
- Keeps existing draft state, dirty state, save, discard, delete, add file, delete file, and workflow tab behavior.

- [ ] **Step 1: Remove permanent metadata sidebar from Content tab**

Delete the right metadata aside in the Content tab only. Do not remove metadata data loading because the compact header still uses creator, origin, file count, used-by count, and permissions.

- [ ] **Step 2: Build compact header inside right panel**

At the top of the right panel, render:

- Editable skill name.
- Compact description textarea, overflow-safe.
- Metadata chips for origin, updated time, creator, file count, used-by count, permission/read-only state, and compact ID.

Use `min-w-0`, `break-words`, and height bounds so description cannot push layout horizontally or vertically.

- [ ] **Step 3: Expand file workspace**

Make the selected file workspace fill remaining right-panel space:

- Left file tree remains compact.
- Right panel is `min-w-0 flex-1`.
- `FileViewer` receives `canEdit={canEdit}` and fills available height.
- Save bar remains at the bottom of the right panel and is not inside file content scroll.

- [ ] **Step 4: Preserve workflow tab**

Do not alter the Workflow tab behavior except for any imports/types made obsolete by Content tab sidebar removal.

---

### Task 3: Add Layout and Regression Tests

**Files:**
- Create: `packages/views/skills/components/file-viewer.test.tsx`
- Create or modify: `packages/views/skills/components/skill-detail-page.test.tsx`

**Interfaces:**
- Tests should mock heavy app dependencies where needed.

- [ ] **Step 1: Test `FileViewer` mode behavior**

Assert:

- Markdown starts in Preview and can switch to Edit when editable.
- Read-only Markdown does not expose an enabled Edit button.
- YAML/JSON/text render with their renderer labels and preserve long content in a preformatted container.

- [ ] **Step 2: Test Skill detail layout**

Render a skill with a very long description and assert:

- Permanent metadata sidebar heading is absent from Content tab.
- Compact metadata chips are present.
- File workspace is present.
- Description control has overflow-safe classes.

- [ ] **Step 3: Run focused tests**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views test -- skills/components/skill-file-renderers.test.tsx skills/components/file-viewer.test.tsx skills/components/skill-detail-page.test.tsx
```

Expected: tests pass.

- [ ] **Step 4: Run typecheck**

Run:

```bash
PATH="$HOME/.nvm/versions/node/v22.22.2/bin:$PATH" pnpm --filter @multica/views typecheck
```

Expected: `tsc --noEmit` passes.

---

## Self-Review

- Spec coverage: layout, description overflow, local renderer registry, Markdown reuse, non-Markdown fallback, edit/save, and verification are covered.
- Scope: UI-only Skills frontend. No backend/runtime/global Markdown changes.
- Risk: `SkillDetailPage` has existing dirty changes in the worktree; edits must preserve those current changes and avoid touching unrelated workflow/server files.
