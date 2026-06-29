"use client";

import { useEffect, useRef, useState } from "react";
import mermaid from "mermaid";

let initialized = false;
let renderSeq = 0;

function ensureMermaid() {
  if (initialized) return;
  mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral" });
  initialized = true;
}

export function MermaidGraph({ chart }: { chart: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    ensureMermaid();
    const id = `wf-mermaid-${renderSeq++}`;

    mermaid.render(id, chart)
      .then(({ svg }) => {
        if (cancelled || !ref.current) return;
        ref.current.innerHTML = svg;
        setError(null);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "render failed");
      });

    return () => {
      cancelled = true;
    };
  }, [chart]);

  if (error) {
    return <div className="text-sm text-destructive">Workflow graph render failed: {error}</div>;
  }

  return <div ref={ref} className="overflow-auto" />;
}
