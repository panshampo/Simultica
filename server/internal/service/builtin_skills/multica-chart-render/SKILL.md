---
name: multica-chart-render
description: "Use when an agent needs to render a tabular dataset into a PNG chart inside its working directory so the platform can later embed the image in a chat reply or comment. Documents the chart_render.py contract end to end: the JSON input schema (columns, rows, chart_type, optional title and output_path), the three supported chart types (bar, line, pie), the 10MB output cap, the workdir-relative output rule, the failure modes that return a non-zero exit and a structured stderr message, and the canonical uv invocation that pins matplotlib and pandas into a workdir-local venv without polluting the host. WHEN to render at all and WHERE to reference the resulting PNG inside the final answer is in the runtime brief's Image Output section, not here."
user-invocable: false
allowed-tools: Bash(uv *), Bash(python3 *)
---

# Chart Render

This skill states WHAT `scripts/chart_render.py` does for a Multica agent
rendering data into a PNG chart, traced to source. WHEN to draw a chart at all
and WHERE to reference it in your final answer is in the runtime brief's
`## Image Output` section; follow that and do not repeat it here.

Every claim below is pinned to source in
`references/chart-render-source-map.md`. If behavior ever differs from this
document, the source map is where to re-check it.

## When to load this skill

Load this skill the moment a task asks you to "render a chart", "draw a graph",
"visualize", "可视化", "图表", or hands you a tabular dataset and expects a
picture in the reply. Do not load it for non-chart image work (screenshots,
icons, photos) — `scripts/chart_render.py` only knows three chart shapes.

## The contract — input

The script reads ONE JSON object from stdin. Required keys:

- `columns` — array of column header strings, length ≥ 1.
- `rows` — array of arrays. Each inner array's length MUST equal
  `len(columns)`. Length must be ≥ 1.
- `chart_type` — one of `"bar"`, `"line"`, `"pie"`. Anything else is a hard
  error.

Optional keys:

- `title` — string. Drawn as the figure title; falls back to no title.
- `output_path` — workdir-relative path ending in `.png` or `.jpg`. Defaults
  to `chart.png` in the current working directory. Must NOT escape the
  current directory (`..` rejected) and must NOT be absolute. The script
  exits non-zero if the resolved path leaves the workdir.
- `x_column` / `y_column` — column names used as the X axis / Y axis for
  `bar` and `line`. Default: `x_column` = `columns[0]`, `y_column` =
  `columns[1]`. For `pie`, `x_column` is the label column and `y_column` is
  the value column with the same defaults.

Example input (typed at stdin):

```json
{
  "columns": ["国家", "GMV"],
  "rows": [["MY", 12.4], ["TH", 9.1], ["VN", 7.8]],
  "chart_type": "bar",
  "title": "大促 GMV (亿)",
  "output_path": "charts/promo-gmv.png"
}
```

## The contract — output

On success the script:

1. Writes the PNG to `<workdir>/<output_path>`, creating intermediate
   directories as needed.
2. Prints exactly one JSON object to stdout:
   ```json
   {"path": "charts/promo-gmv.png", "bytes": 24317, "chart_type": "bar"}
   ```
3. Exits with code `0`.

The `path` field is workdir-relative — the same string you should drop into
the runtime brief's required `![alt](./<path>)` reference in your final
answer.

The output PNG MUST be ≤ 10MB. If matplotlib produces a larger file, the
script deletes it and exits with the `image_too_large` error below — agents
must downscale (drop rows, reduce DPI by passing fewer rows) and retry.

## The contract — failure modes

On any error the script prints exactly one JSON object to stderr and exits
with a non-zero code. Recognized error codes (the `error` field):

| code | meaning |
| --- | --- |
| `invalid_json` | stdin was not parseable JSON |
| `missing_field` | required key absent or wrong type |
| `unsupported_chart_type` | `chart_type` not in `bar` / `line` / `pie` |
| `empty_dataset` | `rows` is `[]` or `columns` is `[]` |
| `row_shape_mismatch` | a row's length ≠ `len(columns)` |
| `unknown_column` | `x_column` / `y_column` not in `columns` |
| `path_escape` | `output_path` is absolute or escapes the workdir |
| `image_too_large` | rendered PNG > 10485760 bytes (10 MiB) |
| `render_failed` | matplotlib raised — full message in `detail` |

Example stderr on failure (followed by exit code 2):

```json
{"error": "row_shape_mismatch", "detail": "row 1 has 3 fields, expected 2"}
```

Treat any non-zero exit as terminal — do not retry the same input. The
`detail` field is human-readable and safe to surface in the failure reply.

## The canonical invocation

The script depends on `matplotlib` and `pandas` pinned in
`scripts/requirements.txt`. The platform's expected runner is `uv`, which
materializes a workdir-local venv on first call and reuses its cache on
subsequent calls — no global `pip install`, no daemon-side bootstrap:

```bash
uv run \
  --with-requirements server/internal/service/.../multica-chart-render/scripts/requirements.txt \
  python3 server/internal/service/.../multica-chart-render/scripts/chart_render.py \
  < input.json
```

In an agent workdir the platform copies the skill payload alongside the
working tree, so the practical call is:

```bash
cat input.json | uv run \
  --with-requirements scripts/chart_render.py.requirements.txt \
  scripts/chart_render.py
```

If `uv` is not available, the script also runs under any plain `python3`
that has matplotlib + pandas importable; do not install them globally — fall
back is a host-prepared venv only.

Do NOT shell-quote the JSON inline. Always pipe from a file or HEREDOC; the
script depends on byte-exact stdin.

## What this skill is NOT for

- Non-tabular images (icons, screenshots, brand assets) — render those by
  whatever means the task supplies; they do not belong to this skill.
- Multi-figure dashboards — render each subchart with one call, then arrange
  references in your final answer.
- HTML / SVG / interactive output — out of scope, the platform's image
  attachment path only ingests PNG / JPG.
- Choosing the chart type for the user — if the input does not name
  `chart_type`, ask the user; do not guess.

## Source map

`references/chart-render-source-map.md` pins every claim above to the line
in `scripts/chart_render.py` that enforces it. When the script changes,
re-derive from the source map first; do not edit this file from memory.
