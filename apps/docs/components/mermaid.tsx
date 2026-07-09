"use client";

import { useEffect, useId, useState } from "react";
import { useTheme } from "next-themes";
import { Maximize2, X } from "lucide-react";

/**
 * Client-side Mermaid diagram renderer.
 *
 * Dynamic-imports the mermaid package so it's only loaded on pages that
 * actually use it (~400 KB). Re-renders when the page theme flips.
 *
 * Themed to pick up Si-Multica design tokens at runtime via getComputedStyle,
 * so the diagram tracks both light / dark mode and any future token changes
 * without a rebuild.
 */
export function Mermaid({ chart }: { chart: string }) {
  const reactId = useId();
  const { resolvedTheme } = useTheme();
  const [svg, setSvg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    let cancelled = false;

    void import("mermaid").then(({ default: mermaid }) => {
      const css = getComputedStyle(document.documentElement);
      // Mermaid's khroma parser only understands legacy color syntax (hex /
      // rgb / hsl / named). Our tokens are authored in oklch(), which
      // getComputedStyle preserves verbatim, and a `color-mix(in srgb, ...)`
      // round-trip still serializes as `color(srgb r g b)` per CSS Color 4.
      // Rasterize each token through a 1x1 canvas: fillStyle accepts any CSS
      // <color>, getImageData returns concrete 8-bit sRGB bytes regardless
      // of the input's color space.
      const canvas = document.createElement("canvas");
      canvas.width = 1;
      canvas.height = 1;
      const ctx = canvas.getContext("2d", { willReadFrequently: true });

      const v = (name: string, fallback: string) => {
        const raw = css.getPropertyValue(name).trim();
        if (!raw || !ctx) return fallback;
        // fillStyle silently ignores unparseable input; prime with a known
        // baseline so a parse failure paints black, not whatever was last set.
        ctx.fillStyle = "#000";
        ctx.fillStyle = raw;
        ctx.fillRect(0, 0, 1, 1);
        const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
        return `rgb(${r}, ${g}, ${b})`;
      };

      const brand = v("--brand", "#3b82f6");
      const brandFg = v("--brand-foreground", "#ffffff");
      const background = v("--background", "#ffffff");
      const foreground = v("--foreground", "#111111");
      const muted = v("--muted", "#f5f5f5");
      const mutedFg = v("--muted-foreground", "#6b7280");
      const border = v("--border", "#e5e5e5");
      const accent = v("--accent", muted);

      mermaid.initialize({
        startOnLoad: false,
        theme: "base",
        securityLevel: "strict",
        fontFamily: "inherit",
        themeVariables: {
          // Canvas
          background,
          mainBkg: background,
          // Nodes — soft muted fill with full-contrast text and a subtle border
          primaryColor: muted,
          primaryTextColor: foreground,
          primaryBorderColor: border,
          secondaryColor: accent,
          secondaryTextColor: foreground,
          secondaryBorderColor: border,
          tertiaryColor: background,
          tertiaryTextColor: foreground,
          tertiaryBorderColor: border,
          // Edges + labels
          lineColor: mutedFg,
          textColor: foreground,
          edgeLabelBackground: background,
          labelBackground: background,
          // Clusters (subgraph boxes)
          clusterBkg: accent,
          clusterBorder: border,
          titleColor: foreground,
          // Notes / callouts
          noteBkgColor: muted,
          noteTextColor: foreground,
          noteBorderColor: border,
          // Brand accent — used for active / start states in state diagrams,
          // user-decision diamonds in flowcharts, etc.
          activeTaskBkgColor: brand,
          activeTaskBorderColor: brand,
          altBackground: muted,
          // Sequence / git diagrams (harmless if unused)
          actorBkg: muted,
          actorBorder: border,
          actorTextColor: foreground,
          actorLineColor: mutedFg,
          signalColor: foreground,
          signalTextColor: foreground,
          // Fine print
          errorBkgColor: muted,
          errorTextColor: foreground,
        },
      });

      // mermaid requires a DOM-valid id; useId returns ":r0:" which isn't.
      const domId = `mermaid-${reactId.replace(/:/g, "")}`;

      mermaid
        .render(domId, chart.trim())
        .then((result) => {
          if (!cancelled) {
            setSvg(result.svg);
            setError(null);
          }
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : String(err));
            setSvg(null);
          }
        });
    });

    return () => {
      cancelled = true;
    };
  }, [chart, reactId, resolvedTheme]);

  if (error) {
    return (
      <pre className="my-4 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
        Mermaid error: {error}
      </pre>
    );
  }

  if (!svg) {
    return (
      <div className="my-4 text-sm text-muted-foreground">
        Rendering diagram…
      </div>
    );
  }

  return (
    <>
      <figure className="not-prose my-6 overflow-hidden rounded-md border border-border/60 bg-muted/20">
        <div className="flex items-center justify-end border-b border-border/60 bg-background/80 px-2 py-1.5">
          <button
            type="button"
            aria-label="Expand diagram"
            className="inline-flex h-7 items-center gap-1 rounded-md px-2 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={() => setExpanded(true)}
          >
            <Maximize2 className="size-3.5" aria-hidden="true" />
            Expand
          </button>
        </div>
        <div
          className="flex justify-center overflow-x-auto p-6 [&_.label_foreignObject>div]:!font-[inherit] [&_.nodeLabel]:!font-[inherit] [&_.edgeLabel]:!font-[inherit] [&_text]:!font-[inherit] [&_svg]:max-w-none"
          dangerouslySetInnerHTML={{ __html: svg }}
        />
      </figure>
      {expanded && (
        <div
          role="dialog"
          aria-label="Expanded diagram"
          className="fixed inset-0 z-[9999] flex flex-col bg-background"
        >
          <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
            <div className="text-sm font-medium">Expanded diagram</div>
            <button
              type="button"
              aria-label="Close expanded diagram"
              className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
              onClick={() => setExpanded(false)}
            >
              <X className="size-4" aria-hidden="true" />
            </button>
          </div>
          <div
            className="min-h-0 flex-1 overflow-auto p-6 [&_.label_foreignObject>div]:!font-[inherit] [&_.nodeLabel]:!font-[inherit] [&_.edgeLabel]:!font-[inherit] [&_text]:!font-[inherit] [&_svg]:mx-auto [&_svg]:h-auto [&_svg]:max-w-none [&_svg]:min-w-[960px]"
            dangerouslySetInnerHTML={{ __html: svg }}
          />
        </div>
      )}
    </>
  );
}
