"use client";

import { use } from "react";
import { useSearchParams } from "next/navigation";
import { WorkflowCaseDetailPage } from "@multica/views/workflow";

export default function Page({
  params,
}: {
  params: Promise<{ caseId: string }>;
}) {
  const { caseId } = use(params);
  const searchParams = useSearchParams();
  const runId = searchParams.get("run_id");
  return <WorkflowCaseDetailPage caseId={caseId} initialRunId={runId} />;
}
