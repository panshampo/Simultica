import { cn } from "@multica/ui/lib/utils";

export type ManagementActionIntent =
  | "create"
  | "execute"
  | "configure"
  | "save";

const INTENT_CLASS: Record<ManagementActionIntent, string> = {
  create:
    "border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100 hover:text-emerald-800 focus-visible:border-emerald-400 focus-visible:ring-emerald-200 dark:border-emerald-900/60 dark:bg-emerald-950/35 dark:text-emerald-300 dark:hover:bg-emerald-900/45",
  execute:
    "border-blue-200 bg-blue-50 text-blue-700 hover:bg-blue-100 hover:text-blue-800 focus-visible:border-blue-400 focus-visible:ring-blue-200 dark:border-blue-900/60 dark:bg-blue-950/35 dark:text-blue-300 dark:hover:bg-blue-900/45",
  configure:
    "border-violet-200 bg-violet-50 text-violet-700 hover:bg-violet-100 hover:text-violet-800 focus-visible:border-violet-400 focus-visible:ring-violet-200 dark:border-violet-900/60 dark:bg-violet-950/35 dark:text-violet-300 dark:hover:bg-violet-900/45",
  save:
    "border-teal-700 bg-teal-600 text-white hover:bg-teal-700 hover:text-white focus-visible:border-teal-500 focus-visible:ring-teal-200 dark:border-teal-500 dark:bg-teal-600 dark:text-white dark:hover:bg-teal-500",
};

export function managementActionButtonClass(
  intent: ManagementActionIntent,
  className?: string,
) {
  return cn(INTENT_CLASS[intent], className);
}
