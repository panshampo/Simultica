# Skill Content Workbench Design

Date: 2026-07-01
Status: Approved design, awaiting implementation plan
Scope: Simultica frontend, Skills management Content tab

## Goal

Improve the skill content detail page so it works as a compact content workbench:

- Fix description overflow in the skill detail content page.
- Remove the permanent right-side metadata column.
- Merge skill title, description, and metadata into a compact basic information panel.
- Give the remaining right-side main panel primarily to file body preview/editing.
- Keep the existing single-pane Preview/Edit switching model; do not introduce split view.
- Add an internal file-type viewer/editor registry so future file renderers can be added without changing the page layout.

This is a UI-only change. It must not change server APIs, skill storage, workflow runtime, agent runtime, database schema, or global Markdown rendering behavior.

## Current Context

The current Skills detail Content tab is structured as:

- Left file tree.
- Middle skill name, description, selected file viewer, save bar.
- Right metadata sidebar with metadata, origin, used-by, and permissions.

The current layout causes two problems:

- Long descriptions can overflow or consume too much space in the middle panel.
- The always-visible metadata sidebar competes with the selected file content, even though the main user task is reading or editing skill files.

The current `FileViewer` already parses Markdown frontmatter, uses a Preview/Edit toggle for Markdown files, and delegates preview rendering to the existing app-level `Markdown` component.

## Layout Design

The Content tab becomes a two-column workbench:

- Left column: file tree and file management.
- Right column: compact skill information panel plus selected file workspace.

The left column keeps the existing file tree behavior:

- `SKILL.md` remains the primary file.
- Add file remains in the file tree column.
- Delete selected non-`SKILL.md` file remains in the file tree column.
- Width should stay compact, around the current `md:w-56` to `240px` range.

The right column is split vertically:

1. `SkillHeaderCompact`
   - Contains skill name.
   - Contains description in a compact area.
   - Contains metadata as small chips or condensed rows.
   - Keeps the panel height small, roughly 96-132px when possible.

2. `SkillFileWorkspace`
   - Occupies the remaining height.
   - Contains file toolbar and selected file viewer/editor.
   - Preview and edit both use the full workspace area.

The permanent metadata sidebar is removed from the Content tab. Its information is folded into the compact header or available through small popovers/tooltips where detail is useful.

## Compact Header

The compact header includes:

- Skill name as the main editable field.
- Skill description as a compact editable field.
- Metadata chips:
  - created
  - updated
  - creator
  - file count
  - origin summary
  - used-by count
  - permission/read-only state

Description overflow must be fixed at both content and layout levels:

- All flex/grid containers in the right panel must use `min-w-0` where needed.
- Description display/editing must support wrapping long text.
- Long URLs and long unbroken words must not expand the page width.
- Description should be height-limited in its compact state.
- If expanded editing is needed, use an explicit affordance such as a popover/dialog or an expanded state, not accidental layout growth.

## File Workspace

The file workspace contains:

- File path.
- File type label.
- Preview/Edit toggle when preview is available.
- Read-only hint or disabled edit control when the skill cannot be edited.
- The selected file body view/editor.

No split view is introduced. Preview and edit continue to be mutually exclusive modes in one large panel.

Recommended defaults:

- Markdown files default to Preview.
- Non-Markdown files may default to Preview when a renderer exists, otherwise Edit/source view is acceptable.
- Mode state is UI-only and not persisted to the database.

The dirty-state and save model remains the same:

- Changing name, description, `SKILL.md`, or extra files marks the skill dirty.
- The existing save bar remains visible when dirty.
- The save bar should stay accessible at the bottom of the content workbench and not be buried inside file body scroll.

## File Renderer Registry

Add a local skills-module file renderer registry. This is an internal extension point, not a runtime plugin system.

Example shape:

```ts
type SkillFileMode = "preview" | "edit";

type SkillFileRenderProps = {
  path: string;
  content: string;
  canEdit: boolean;
  onChange: (content: string) => void;
};

type SkillFileRenderer = {
  id: string;
  label: string;
  matches: (path: string) => boolean;
  defaultMode: SkillFileMode;
  canPreview: boolean;
  Preview?: React.ComponentType<SkillFileRenderProps>;
  Editor: React.ComponentType<SkillFileRenderProps>;
};
```

Initial renderers:

- Markdown renderer for `.md` and `.mdx`.
- YAML renderer for `.yaml` and `.yml`.
- JSON renderer for `.json`.
- Text fallback renderer for all other files.

The registry is intentionally local to `packages/views/skills/components` or a nearby `skills/lib` file. It must not change the global Markdown component, app-wide renderer behavior, backend APIs, or persisted skill model.

## Markdown Renderer

The Markdown renderer should continue using the existing app-level Markdown path:

- `packages/views/common/markdown.tsx`
- underlying `@multica/ui/markdown`

The renderer should improve the skill-specific presentation around that component:

- Render frontmatter in a compact `MarkdownFrontmatterPanel`.
- Render body inside a `SkillMarkdownDocument` container.
- Add layout-safe styles for long text, long URLs, tables, and code blocks.

Supported content should include:

- Headings.
- Paragraphs.
- Bold and italic text.
- Inline code.
- Code blocks.
- GFM tables.
- Task lists.
- Blockquotes.
- Images and file cards through the existing app renderer.
- Math when already supported by the existing Markdown stack.
- Sanitized raw HTML according to existing Markdown behavior.

The first version should not add:

- A new Markdown parser.
- Global Markdown behavior changes.
- Mermaid rendering.
- Heading table of contents.
- Reading progress.
- Full document search.

## YAML, JSON, and Text Renderers

YAML and JSON renderers are deliberately simple in the first version:

- Preview renders formatted or pre-wrapped code with readable styling.
- Edit renders a monospace textarea.
- No schema validation.
- No workflow-specific semantics.

The text fallback renderer:

- Preview renders `pre-wrap` text in a layout-safe container.
- Edit renders a monospace textarea.

Long lines in code/text preview must not break the page layout. Use horizontal scrolling inside the preview surface where appropriate.

## Out Of Scope

This design does not include:

- Backend API changes.
- Skill storage format changes.
- Database changes.
- Agent/runtime/workflow behavior changes.
- Global Markdown renderer changes.
- Split view editing.
- Mermaid support.
- Documentation library features such as outline, anchors, search, and reading progress.
- Workflow tab redesign.

## Test Plan

Focused tests should cover:

1. Layout
   - Content tab renders without the permanent right metadata sidebar.
   - Compact header shows skill name, description, and metadata summaries.
   - File workspace remains present and owns the main content area.

2. Description overflow
   - Very long description text does not expand the page width.
   - Long URLs and long unbroken words are wrapped or clipped safely.
   - Description remains bounded in the compact header.

3. Renderer registry
   - `.md` and `.mdx` select the Markdown renderer.
   - `.yaml` and `.yml` select the YAML renderer.
   - `.json` selects the JSON renderer.
   - Unknown extensions select the text fallback renderer.

4. Markdown preview
   - Frontmatter is rendered separately.
   - Markdown body is passed through the existing app-level Markdown renderer.
   - Tables and code blocks are contained by overflow-safe wrappers.

5. Edit/save regression
   - Editing `SKILL.md` marks the skill dirty.
   - Editing name or description marks the skill dirty.
   - Switching files does not lose draft file content.
   - Read-only skills do not allow editing.

6. Verification
   - Run focused skills tests.
   - Run `pnpm --filter @multica/views typecheck` with Node 22.

## Implementation Notes

Likely files:

- `packages/views/skills/components/skill-detail-page.tsx`
- `packages/views/skills/components/file-viewer.tsx`
- New local renderer registry file under `packages/views/skills/components` or `packages/views/skills/lib`.
- Skill detail tests if present, or new focused tests around extracted components.
- Locale files only if new visible labels need translation.

Use existing UI components and styling patterns. Keep the change scoped to the Skills frontend surface.
