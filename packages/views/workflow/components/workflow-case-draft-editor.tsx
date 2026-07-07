"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import type { WorkflowDefinition, WorkflowDefinitionDraft } from "@multica/core/workflow/types";
import { api } from "@multica/core/api";
import { Button } from "@multica/ui/components/ui/button";
import { parseWorkflow, serializeWorkflow } from "../lib/serialize";
import { WorkflowYamlEditor } from "./workflow-yaml-editor";

export const WORKFLOW_CASE_STARTER_DRAFT: WorkflowDefinition = {
  meta: { name: "workflow-case", version: "1" },
  state: {
    fields: [{ name: "task", type: "string", required: true }],
  },
  nodes: [
    {
      id: "gate",
      type: "condition",
      dispatch: "inline",
      config: {
        condition: "true",
        output: "route_decision",
      },
    },
  ],
  routing: [
    { from: "START", to: "gate" },
    { from: "gate", to: "END" },
  ],
};

export function WorkflowCaseDraftEditor({
  caseId,
  definition,
  onSaved,
  onCancel,
}: {
  caseId: string;
  definition: WorkflowDefinitionDraft | null | undefined;
  onSaved: (definition: WorkflowDefinitionDraft) => void;
  onCancel: () => void;
}) {
  const [yaml, setYaml] = useState(() => serializeWorkflow(definition?.draft_json ?? WORKFLOW_CASE_STARTER_DRAFT));
  const [yamlError, setYamlError] = useState<string | null>(null);
  const [parsed, setParsed] = useState<WorkflowDefinition | null>(definition?.draft_json ?? WORKFLOW_CASE_STARTER_DRAFT);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const next = definition?.draft_json ?? WORKFLOW_CASE_STARTER_DRAFT;
    setYaml(serializeWorkflow(next));
    setParsed(next);
    setYamlError(null);
  }, [definition]);

  async function saveDraft() {
    let draft = parsed;
    if (!draft) {
      try {
        draft = parseWorkflow(yaml);
        setParsed(draft);
        setYamlError(null);
      } catch (err) {
        setYamlError(`YAML parse failed: ${err instanceof Error ? err.message : "unknown error"}`);
        return;
      }
    }

    setSaving(true);
    try {
      const saved = await api.upsertWorkflowCaseDefinitionDraft(caseId, {
        draft_json: draft,
        source_templates: definition?.source_templates ?? [],
      });
      toast.success("Draft saved");
      onSaved(saved);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save draft");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-3 rounded-lg border bg-card p-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="text-sm font-medium">{definition ? "Edit draft" : "Create starter draft"}</h3>
          <p className="mt-1 text-xs text-muted-foreground">
            Edit the workflow YAML, apply it, then save the draft before validation.
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button type="button" size="sm" variant="outline" onClick={onCancel} disabled={saving}>
            Cancel
          </Button>
          <Button type="button" size="sm" onClick={() => void saveDraft()} disabled={saving || Boolean(yamlError)}>
            {saving ? "Saving..." : "Save draft"}
          </Button>
        </div>
      </div>
      <WorkflowYamlEditor
        yaml={yaml}
        error={yamlError}
        onYamlChange={(next) => {
          setYaml(next);
          setParsed(null);
        }}
        onError={setYamlError}
        onParsed={(next, nextYaml) => {
          setParsed(next);
          setYaml(nextYaml);
        }}
      />
    </div>
  );
}
