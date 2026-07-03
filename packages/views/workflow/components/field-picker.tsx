import type { WorkflowStateField } from "@multica/core/workflow/types";

export function FieldPicker({
  label,
  value,
  fields,
  allowedTypes,
  onChange,
  onCreateField,
}: {
  label: string;
  value: string;
  fields: WorkflowStateField[];
  allowedTypes?: string[];
  onChange: (fieldName: string) => void;
  onCreateField?: (name: string) => void;
}) {
  const visibleFields = allowedTypes?.length
    ? fields.filter((field) => allowedTypes.includes(field.type))
    : fields;

  return (
    <label className="space-y-1 text-xs text-muted-foreground">
      <span>{label}</span>
      <select
        className="h-9 w-full rounded-md border bg-background px-2 text-sm text-foreground"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">Select field</option>
        {visibleFields.map((field) => (
          <option key={field.name} value={field.name}>
            {field.name} · {field.type}
          </option>
        ))}
      </select>
      {onCreateField && value && !fields.some((field) => field.name === value) && (
        <button type="button" className="text-xs text-primary" onClick={() => onCreateField(value)}>
          Create field "{value}"
        </button>
      )}
    </label>
  );
}
