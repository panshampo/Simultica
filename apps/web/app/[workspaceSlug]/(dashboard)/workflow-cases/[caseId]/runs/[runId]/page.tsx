"use client";

import { use } from "react";
import { useSearchParams } from "next/navigation";
import { WorkflowRunDetailPage } from "@multica/views/workflow";

export default function Page({
  params,
}: {
  params: Promise<{ caseId: string; runId: string }>;
}) {
  const { caseId, runId } = use(params);
  const searchParams = useSearchParams();
  const nodeId = searchParams.get("node_id");
  return <WorkflowRunDetailPage caseId={caseId} runId={runId} initialNodeId={nodeId} />;
}
