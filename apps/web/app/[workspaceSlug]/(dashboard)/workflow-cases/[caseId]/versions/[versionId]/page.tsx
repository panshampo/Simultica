"use client";

import { use } from "react";
import { WorkflowCaseVersionPage } from "@multica/views/workflow";

export default function Page({
  params,
}: {
  params: Promise<{ caseId: string; versionId: string }>;
}) {
  const { caseId, versionId } = use(params);
  return <WorkflowCaseVersionPage caseId={caseId} versionId={versionId} />;
}
