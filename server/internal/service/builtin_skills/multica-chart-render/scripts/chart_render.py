#!/usr/bin/env python3
"""chart_render.py — Multica daemon-side chart renderer.

Reads ONE JSON object from stdin, renders a PNG chart inside the current
working directory, prints a one-line JSON success descriptor to stdout, and
exits 0. Any error path prints a one-line JSON error to stderr and exits
non-zero. The contract is documented in SKILL.md alongside this script.
"""

from __future__ import annotations

import json
import os
import sys
from typing import Any, Dict, List, Tuple


MAX_BYTES = 10 * 1024 * 1024  # 10 MiB hard cap on rendered PNG/JPG.
SUPPORTED_CHART_TYPES = ("bar", "line", "pie")
SUPPORTED_EXTENSIONS = (".png", ".jpg", ".jpeg")


class RenderError(Exception):
    """Structured error surface — caller decides exit code + stderr shape."""

    def __init__(self, code: str, detail: str) -> None:
        super().__init__(detail)
        self.code = code
        self.detail = detail


def _fail(code: str, detail: str) -> None:
    json.dump({"error": code, "detail": detail}, sys.stderr)
    sys.stderr.write("\n")
    sys.exit(2)


def _read_input(stream) -> Dict[str, Any]:
    raw = stream.read()
    if not raw.strip():
        raise RenderError("invalid_json", "stdin was empty")
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as e:
        raise RenderError("invalid_json", f"stdin is not valid JSON: {e}")
    if not isinstance(payload, dict):
        raise RenderError("invalid_json", "stdin must be a JSON object")
    return payload


def _require(payload: Dict[str, Any], key: str, expected_type: type) -> Any:
    if key not in payload:
        raise RenderError("missing_field", f"missing required key: {key}")
    value = payload[key]
    if not isinstance(value, expected_type):
        raise RenderError(
            "missing_field",
            f"key {key!r} must be of type {expected_type.__name__}",
        )
    return value


def _validate_dataset(
    columns: List[str], rows: List[List[Any]]
) -> Tuple[List[str], List[List[Any]]]:
    if not columns:
        raise RenderError("empty_dataset", "columns is empty")
    if not rows:
        raise RenderError("empty_dataset", "rows is empty")
    for i, c in enumerate(columns):
        if not isinstance(c, str) or not c:
            raise RenderError(
                "missing_field", f"columns[{i}] must be a non-empty string"
            )
    for i, row in enumerate(rows):
        if not isinstance(row, list):
            raise RenderError(
                "row_shape_mismatch", f"row {i} is not a JSON array"
            )
        if len(row) != len(columns):
            raise RenderError(
                "row_shape_mismatch",
                f"row {i} has {len(row)} fields, expected {len(columns)}",
            )
    return columns, rows


def _resolve_output_path(payload: Dict[str, Any]) -> str:
    raw = payload.get("output_path", "chart.png")
    if not isinstance(raw, str) or not raw:
        raise RenderError("missing_field", "output_path must be a non-empty string")
    if os.path.isabs(raw):
        raise RenderError("path_escape", "output_path must be relative, not absolute")
    normalized = os.path.normpath(raw)
    if normalized.startswith("..") or normalized == "..":
        raise RenderError("path_escape", "output_path escapes the working directory")
    if not normalized.lower().endswith(SUPPORTED_EXTENSIONS):
        raise RenderError(
            "missing_field",
            "output_path must end with .png / .jpg / .jpeg",
        )
    return normalized


def _resolve_axes(
    payload: Dict[str, Any], columns: List[str]
) -> Tuple[str, str]:
    x = payload.get("x_column", columns[0])
    y = payload.get("y_column", columns[1] if len(columns) > 1 else columns[0])
    if not isinstance(x, str) or not isinstance(y, str):
        raise RenderError("missing_field", "x_column / y_column must be strings")
    if x not in columns:
        raise RenderError("unknown_column", f"x_column {x!r} not in columns")
    if y not in columns:
        raise RenderError("unknown_column", f"y_column {y!r} not in columns")
    return x, y


def _render(
    chart_type: str,
    columns: List[str],
    rows: List[List[Any]],
    x_column: str,
    y_column: str,
    title: str,
    output_path: str,
) -> None:
    # Imported lazily so unit tests that exercise validation don't pay the
    # matplotlib import cost when they shouldn't reach the render branch.
    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import pandas as pd

    df = pd.DataFrame(rows, columns=columns)
    fig, ax = plt.subplots(figsize=(8, 5), dpi=120)
    try:
        if chart_type == "bar":
            ax.bar(df[x_column].astype(str), df[y_column])
            ax.set_xlabel(x_column)
            ax.set_ylabel(y_column)
        elif chart_type == "line":
            ax.plot(df[x_column], df[y_column], marker="o")
            ax.set_xlabel(x_column)
            ax.set_ylabel(y_column)
        elif chart_type == "pie":
            ax.pie(df[y_column], labels=df[x_column].astype(str), autopct="%1.1f%%")
            ax.axis("equal")
        else:
            raise RenderError(
                "unsupported_chart_type",
                f"chart_type {chart_type!r} is not supported",
            )

        if title:
            ax.set_title(title)
        fig.tight_layout()

        parent = os.path.dirname(output_path)
        if parent:
            os.makedirs(parent, exist_ok=True)

        try:
            fig.savefig(output_path)
        except Exception as e:  # noqa: BLE001 — matplotlib surfaces many types
            raise RenderError("render_failed", f"matplotlib.savefig failed: {e}")
    finally:
        plt.close(fig)


def main(argv: List[str], stdin, stdout, stderr) -> int:
    try:
        payload = _read_input(stdin)
        columns = _require(payload, "columns", list)
        rows = _require(payload, "rows", list)
        chart_type = _require(payload, "chart_type", str)
        if chart_type not in SUPPORTED_CHART_TYPES:
            raise RenderError(
                "unsupported_chart_type",
                f"chart_type {chart_type!r} not in {list(SUPPORTED_CHART_TYPES)}",
            )

        columns, rows = _validate_dataset(columns, rows)
        x_column, y_column = _resolve_axes(payload, columns)
        title = payload.get("title", "")
        if title is not None and not isinstance(title, str):
            raise RenderError("missing_field", "title must be a string when present")
        output_path = _resolve_output_path(payload)

        _render(chart_type, columns, rows, x_column, y_column, title or "", output_path)

        size = os.path.getsize(output_path)
        if size > MAX_BYTES:
            try:
                os.remove(output_path)
            except OSError:
                pass
            raise RenderError(
                "image_too_large",
                f"rendered image is {size} bytes, exceeds {MAX_BYTES}",
            )

        json.dump(
            {"path": output_path, "bytes": size, "chart_type": chart_type},
            stdout,
        )
        stdout.write("\n")
        return 0
    except RenderError as e:
        json.dump({"error": e.code, "detail": e.detail}, stderr)
        stderr.write("\n")
        return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv, sys.stdin, sys.stdout, sys.stderr))
