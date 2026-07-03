export type ConditionOperator = "==" | "!=" | "<" | ">" | "<=" | ">=" | "truthy" | "falsy";

export type ConditionRule = {
  field: string;
  operator: ConditionOperator;
  value?: string | number | boolean;
  valueType?: "string" | "number" | "boolean";
};

export type ConditionGroup = {
  join: "AND" | "OR";
  rules: ConditionRule[];
};

export function conditionGroupToExpression(group: ConditionGroup): string {
  const joiner = group.join === "AND" ? " && " : " || ";
  return group.rules.map(ruleToExpression).join(joiner);
}

export function tryParseConditionExpression(expression: string): ConditionGroup | null {
  const trimmed = expression.trim();
  if (!trimmed || trimmed.includes("(") || trimmed.includes(")")) return null;
  const join = trimmed.includes(" || ") ? "OR" : "AND";
  const splitter = join === "OR" ? " || " : " && ";
  const parts = trimmed.split(splitter).map((part) => part.trim()).filter(Boolean);
  const rules = parts.map(parseRule);
  if (rules.some((rule) => rule == null)) return null;
  return { join, rules: rules as ConditionRule[] };
}

function ruleToExpression(rule: ConditionRule): string {
  if (rule.operator === "truthy") return rule.field;
  if (rule.operator === "falsy") return `!${rule.field}`;
  return `${rule.field} ${rule.operator} ${formatValue(rule.value, rule.valueType)}`;
}

function formatValue(value: ConditionRule["value"], valueType: ConditionRule["valueType"]): string {
  if (valueType === "number") return String(Number(value ?? 0));
  if (valueType === "boolean") return String(Boolean(value));
  return `"${String(value ?? "").replaceAll('"', '\\"')}"`;
}

function parseRule(part: string): ConditionRule | null {
  if (/^![A-Za-z_][A-Za-z0-9_]*$/.test(part)) {
    return { field: part.slice(1), operator: "falsy" };
  }
  if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(part)) {
    return { field: part, operator: "truthy" };
  }
  const match = part.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*(==|!=|<=|>=|<|>)\s*(.+)$/);
  if (!match) return null;
  const [, field, operator, rawValue] = match;
  if (!field || !operator || rawValue === undefined) return null;
  const parsed = parseValue(rawValue.trim());
  return { field, operator: operator as ConditionOperator, ...parsed };
}

function parseValue(raw: string): Pick<ConditionRule, "value" | "valueType"> {
  if (raw === "true") return { value: true, valueType: "boolean" };
  if (raw === "false") return { value: false, valueType: "boolean" };
  if (/^-?\d+(\.\d+)?$/.test(raw)) return { value: Number(raw), valueType: "number" };
  const quoted = raw.match(/^["'](.*)["']$/);
  return { value: quoted ? quoted[1] : raw, valueType: "string" };
}
