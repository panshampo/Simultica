import type { WorkflowValidationIssue } from "../lib/validation";

export function WorkflowValidationPanel({ issues }: { issues: WorkflowValidationIssue[] }) {
  if (issues.length === 0) {
    return <div className="rounded-md border border-green-200 bg-green-50 px-3 py-2 text-xs text-green-800">Workflow looks valid.</div>;
  }

  return (
    <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-950">
      <div className="font-medium">{issues.length} workflow issue{issues.length === 1 ? "" : "s"}</div>
      <ul className="mt-1 list-disc space-y-1 pl-4">
        {issues.map((issue) => (
          <li key={`${issue.path}:${issue.message}`}>
            <span className="font-mono">{issue.path}</span>: {issue.message}
          </li>
        ))}
      </ul>
    </div>
  );
}
