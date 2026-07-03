"use client";

import { useMemo } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { yaml } from "@codemirror/lang-yaml";
import { json } from "@codemirror/lang-json";
import { javascript } from "@codemirror/lang-javascript";
import { python } from "@codemirror/lang-python";
import { sql } from "@codemirror/lang-sql";
import { StreamLanguage } from "@codemirror/language";
import { go } from "@codemirror/legacy-modes/mode/go";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { toml } from "@codemirror/legacy-modes/mode/toml";
import { oneDark } from "@codemirror/theme-one-dark";
import { EditorView } from "@codemirror/view";
import { useTheme } from "@multica/ui/components/common/theme-provider";

export type CodeEditorLanguage =
  | "yaml"
  | "json"
  | "typescript"
  | "javascript"
  | "python"
  | "go"
  | "shell"
  | "toml"
  | "sql"
  | "text";

export function CodeEditor({
  value,
  onChange,
  language = "text",
  readOnly = false,
}: {
  value: string;
  onChange?: (value: string) => void;
  language?: CodeEditorLanguage;
  readOnly?: boolean;
}) {
  const { theme } = useTheme();
  const isDark = theme === "dark";

  const extensions = useMemo(() => {
    const exts: any[] = [];
    switch (language) {
      case "yaml":
        exts.push(yaml());
        break;
      case "json":
        exts.push(json());
        break;
      case "typescript":
      case "javascript":
        exts.push(javascript({ typescript: language === "typescript", jsx: true }));
        break;
      case "python":
        exts.push(python());
        break;
      case "go":
        exts.push(StreamLanguage.define(go));
        break;
      case "shell":
        exts.push(StreamLanguage.define(shell));
        break;
      case "toml":
        exts.push(StreamLanguage.define(toml));
        break;
      case "sql":
        exts.push(sql());
        break;
    }
    exts.push(EditorView.lineWrapping);
    if (isDark) exts.push(oneDark);
    return exts;
  }, [language, isDark]);

  const hasLanguage = language !== "text";

  return (
    <CodeMirror
      value={value}
      height="100%"
      extensions={extensions}
      readOnly={readOnly}
      onChange={(val) => onChange?.(val)}
      basicSetup={{
        lineNumbers: true,
        highlightActiveLineGutter: true,
        highlightActiveLine: true,
        foldGutter: hasLanguage,
        bracketMatching: hasLanguage,
        indentOnInput: hasLanguage,
        autocompletion: hasLanguage,
        highlightSelectionMatches: true,
      }}
      className="h-full"
      theme="none"
    />
  );
}
