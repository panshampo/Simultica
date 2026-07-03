"use client";

import type { ComponentType } from "react";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  parseFrontmatter,
  type SkillFrontmatter,
} from "@multica/core/skills/frontmatter";
import { Markdown } from "../../common/markdown";
import { CodeEditor, type CodeEditorLanguage } from "./code-editor";

export type SkillFileMode = "preview" | "edit";

export type SkillFileRenderProps = {
  path: string;
  content: string;
  canEdit: boolean;
  onChange: (content: string) => void;
};

export type SkillFileRenderer = {
  id: string;
  label: string;
  matches: (path: string) => boolean;
  defaultMode: SkillFileMode;
  canPreview: boolean;
  Preview?: ComponentType<SkillFileRenderProps>;
  Editor: ComponentType<SkillFileRenderProps>;
};

export function isMarkdownFile(path: string): boolean {
  const normalized = path.toLowerCase();
  return normalized.endsWith(".md") || normalized.endsWith(".mdx");
}

function hasExtension(path: string, exts: string[]): boolean {
  const normalized = path.toLowerCase();
  return exts.some((ext) => normalized.endsWith(ext));
}

function createCodeMirrorRenderer(
  id: string,
  label: string,
  language: CodeEditorLanguage,
  matches: (path: string) => boolean,
): SkillFileRenderer {
  return {
    id,
    label,
    matches,
    defaultMode: "edit",
    canPreview: true,
    Preview: ({ content }) => <CodeEditor value={content} language={language} readOnly />,
    Editor: ({ content, onChange }) => (
      <CodeEditor value={content} onChange={onChange} language={language} />
    ),
  };
}

function FrontmatterPanel({ data }: { data: SkillFrontmatter }) {
  const entries = Object.entries(data);
  if (entries.length === 0) return null;

  return (
    <div className="mb-5 overflow-hidden rounded-lg border border-border/70 bg-muted/25">
      <div className="border-b px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        Frontmatter
      </div>
      <div className="divide-y divide-border/60">
        {entries.map(([key, value]) => (
          <div key={key} className="grid gap-1 px-3 py-2 text-xs sm:grid-cols-[8rem_minmax(0,1fr)]">
            <div className="min-w-0 font-mono font-medium text-muted-foreground">{key}</div>
            <div className="min-w-0 whitespace-pre-wrap break-words text-foreground">
              {value.trimEnd()}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function SkillFileTextarea({
  content,
  onChange,
  placeholder,
}: Pick<SkillFileRenderProps, "content" | "onChange"> & { placeholder: string }) {
  return (
    <Textarea
      value={content}
      onChange={(event) => onChange(event.target.value)}
      placeholder={placeholder}
      className="h-full min-h-full resize-none rounded-none border-0 bg-background px-4 py-4 font-mono text-sm leading-relaxed focus-visible:ring-0"
    />
  );
}

function MarkdownPreview({ content }: SkillFileRenderProps) {
  const { frontmatter, body } = parseFrontmatter(content);

  return (
    <div className="h-full overflow-y-auto">
      <article className="mx-auto w-full max-w-5xl px-5 py-6">
        {frontmatter && <FrontmatterPanel data={frontmatter} />}
        <div className="min-w-0 overflow-hidden rounded-lg border border-border/70 bg-card/80 px-5 py-5 shadow-[0_1px_1px_rgba(15,23,42,0.03)]">
          <div className="min-w-0 break-words [&_pre]:max-w-full [&_pre]:overflow-x-auto [&_table]:block [&_table]:max-w-full [&_table]:overflow-x-auto">
            <Markdown mode="full">
              {body || "*No content yet*"}
            </Markdown>
          </div>
        </div>
      </article>
    </div>
  );
}

function MarkdownEditor(props: SkillFileRenderProps) {
  return <SkillFileTextarea {...props} placeholder="Write markdown content..." />;
}

function PreformattedPreview({ content }: SkillFileRenderProps) {
  return (
    <div className="h-full overflow-auto bg-background">
      <pre className="min-h-full w-max min-w-full whitespace-pre px-4 py-4 font-mono text-sm leading-relaxed text-foreground">
        {content || "No content yet"}
      </pre>
    </div>
  );
}

function RawEditor(props: SkillFileRenderProps) {
  return <SkillFileTextarea {...props} placeholder="File content..." />;
}

export const skillFileRenderers: SkillFileRenderer[] = [
  {
    id: "markdown",
    label: "Markdown",
    matches: isMarkdownFile,
    defaultMode: "preview",
    canPreview: true,
    Preview: MarkdownPreview,
    Editor: MarkdownEditor,
  },
  createCodeMirrorRenderer(
    "yaml",
    "YAML",
    "yaml",
    (p) => hasExtension(p, [".yaml", ".yml"]),
  ),
  createCodeMirrorRenderer(
    "json",
    "JSON",
    "json",
    (p) => hasExtension(p, [".json", ".jsonc", ".json5"]),
  ),
  createCodeMirrorRenderer(
    "typescript",
    "TypeScript",
    "typescript",
    (p) => hasExtension(p, [".ts", ".tsx"]),
  ),
  createCodeMirrorRenderer(
    "javascript",
    "JavaScript",
    "javascript",
    (p) => hasExtension(p, [".js", ".jsx", ".mjs", ".cjs"]),
  ),
  createCodeMirrorRenderer(
    "python",
    "Python",
    "python",
    (p) => hasExtension(p, [".py"]),
  ),
  createCodeMirrorRenderer(
    "go",
    "Go",
    "go",
    (p) => hasExtension(p, [".go"]),
  ),
  createCodeMirrorRenderer(
    "shell",
    "Shell",
    "shell",
    (p) => hasExtension(p, [".sh", ".bash", ".zsh"]),
  ),
  createCodeMirrorRenderer(
    "toml",
    "TOML",
    "toml",
    (p) => hasExtension(p, [".toml"]),
  ),
  createCodeMirrorRenderer(
    "sql",
    "SQL",
    "sql",
    (p) => hasExtension(p, [".sql"]),
  ),
  {
    id: "text",
    label: "Text",
    matches: () => true,
    defaultMode: "preview",
    canPreview: true,
    Preview: PreformattedPreview,
    Editor: RawEditor,
  },
];

export function getSkillFileRenderer(path: string): SkillFileRenderer {
  return skillFileRenderers.find((renderer) => renderer.matches(path)) ?? skillFileRenderers[skillFileRenderers.length - 1]!;
}
