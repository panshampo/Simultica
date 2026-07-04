import type { WorkflowDefinition } from "@multica/core/workflow/types";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { parseWorkflow } from "../lib/serialize";

export function WorkflowYamlEditor({
  yaml,
  error,
  onYamlChange,
  onError,
  onParsed,
}: {
  yaml: string;
  error: string | null;
  onYamlChange: (yaml: string) => void;
  onError: (error: string | null) => void;
  onParsed: (definition: WorkflowDefinition, yaml: string) => void;
}) {
  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <Textarea
        aria-label="Workflow YAML"
        className="min-h-[24rem] flex-1 font-mono text-xs"
        value={yaml}
        onChange={(event) => onYamlChange(event.target.value)}
      />
      {error && <div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">{error}</div>}
      <Button
        type="button"
        size="sm"
        onClick={() => {
          try {
            const parsed = parseWorkflow(yaml);
            onError(null);
            onParsed(parsed, yaml);
          } catch (err) {
            onError(`YAML parse failed: ${err instanceof Error ? err.message : "unknown error"}`);
          }
        }}
      >
        Apply YAML
      </Button>
    </div>
  );
}
