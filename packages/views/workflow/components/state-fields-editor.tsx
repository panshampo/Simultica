import { useState } from "react";
import type { WorkflowDefinition, WorkflowStateField } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { managementActionButtonClass } from "../../common/management-action-button";
import { addStateField, createStateField, findStateFieldReferences, removeStateField } from "../lib/editor-model";

const FIELD_TYPES: WorkflowStateField["type"][] = ["string", "number", "boolean", "object", "array", "enum"];

const FIELD_TYPE_LABELS: Record<string, string> = {
  string: "String",
  number: "Number",
  boolean: "Boolean",
  object: "Object",
  array: "Array",
  enum: "Enum",
};

export function StateFieldsEditor({
  definition,
  onChange,
}: {
  definition: WorkflowDefinition;
  onChange: (definition: WorkflowDefinition) => void;
}) {
  const [name, setName] = useState("");
  const [type, setType] = useState<WorkflowStateField["type"]>("string");
  const [pendingDelete, setPendingDelete] = useState<{ field: string; references: string[] } | null>(null);

  return (
    <section className="space-y-3 rounded-md border p-3">
      <div>
        <h4 className="text-sm font-medium">State fields</h4>
        <p className="text-xs text-muted-foreground">
          State fields are the workflow&apos;s shared data. Nodes write outputs here, and conditions read from here.
        </p>
      </div>

      <div className="space-y-2">
        {definition.state.fields.map((field) => (
          <div key={field.name} className="flex items-center gap-2 rounded-md border bg-muted/10 px-2 py-1.5">
            <span className="min-w-0 flex-1 truncate font-mono text-xs">{field.name}</span>
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] uppercase text-muted-foreground">{field.type}</span>
            <Button
              type="button"
              size="xs"
              variant="ghost"
              aria-label={`Delete field ${field.name}`}
              onClick={() => {
                const references = findStateFieldReferences(definition, field.name);
                if (references.length > 0) {
                  setPendingDelete({ field: field.name, references });
                  return;
                }
                onChange(removeStateField(definition, field.name));
              }}
            >
              Delete
            </Button>
          </div>
        ))}
      </div>

      {pendingDelete && (
        <div className="rounded-md border border-amber-300 bg-amber-50 p-2 text-xs text-amber-900">
          <p className="font-medium">This field is still used:</p>
          <ul className="mt-1 list-disc pl-4">
            {pendingDelete.references.map((reference) => <li key={reference}>{reference}</li>)}
          </ul>
          <Button type="button" size="xs" variant="outline" className="mt-2" onClick={() => setPendingDelete(null)}>
            Keep field
          </Button>
        </div>
      )}

      <div className="grid grid-cols-[minmax(0,1fr)_8rem_auto] gap-2">
        <Input aria-label="New field name" value={name} onChange={(event) => setName(event.target.value)} placeholder="review_result" />
        <select
          aria-label="New field type"
          className="h-9 rounded-md border bg-background px-2 text-sm"
          value={type}
          onChange={(event) => setType(event.target.value as WorkflowStateField["type"])}
        >
          {FIELD_TYPES.map((fieldType) => <option key={fieldType} value={fieldType}>{FIELD_TYPE_LABELS[fieldType] ?? fieldType}</option>)}
        </select>
        <Button
          type="button"
          size="sm"
          variant="outline"
          className={managementActionButtonClass("create")}
          onClick={() => {
            const field = createStateField(name, type);
            const next = addStateField(definition, field);
            onChange(next);
            if (next !== definition) setName("");
          }}
        >
          Add field
        </Button>
      </div>
    </section>
  );
}
