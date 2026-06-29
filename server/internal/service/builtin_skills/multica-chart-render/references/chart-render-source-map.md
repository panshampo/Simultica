# Chart render — source map

Every claim in `SKILL.md` traces to a line below. Re-derive against the current
script before trusting any line number; the behavior is the contract, the line
is a pointer.

## Input contract

| Fact | Source |
| --- | --- |
| stdin must be one JSON object | `scripts/chart_render.py` `_read_input` |
| `columns` required, list of non-empty strings | `_require` + `_validate_dataset` |
| `rows` required, list of lists, length matches `columns` | `_validate_dataset` |
| `chart_type` required, ∈ `{bar, line, pie}` | `main` + `SUPPORTED_CHART_TYPES` |
| `title` optional string | `main` |
| `output_path` optional, relative, suffix in `.png` / `.jpg` / `.jpeg` | `_resolve_output_path` |
| `x_column` / `y_column` optional, default to columns[0]/columns[1] | `_resolve_axes` |

## Output contract

| Fact | Source |
| --- | --- |
| stdout = single-line JSON `{path, bytes, chart_type}` on success | `main` final `json.dump` |
| exit code 0 on success | `main` returns `0` |
| stderr = single-line JSON `{error, detail}` on failure | `main` `RenderError` branch |
| exit code 2 on failure | `_fail` and `main` |
| PNG > 10MiB triggers `image_too_large` and deletes the file | `MAX_BYTES` + `main` size check |

## Error codes

| Code | Trigger |
| --- | --- |
| `invalid_json` | empty stdin or `JSONDecodeError` |
| `missing_field` | required key absent or wrong type |
| `unsupported_chart_type` | `chart_type` not in supported set |
| `empty_dataset` | `columns` or `rows` is empty |
| `row_shape_mismatch` | row length ≠ `len(columns)` |
| `unknown_column` | `x_column` or `y_column` not in `columns` |
| `path_escape` | `output_path` absolute or escapes workdir |
| `image_too_large` | rendered image exceeds `MAX_BYTES` |
| `render_failed` | matplotlib raised during `savefig` |
