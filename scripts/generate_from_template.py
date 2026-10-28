#!/usr/bin/env python3
"""Render a template with {{ key }} placeholders.

Usage:
  python3 scripts/generate_from_template.py TEMPLATE OUTPUT --var key=value [--force]

Go-style placeholders such as {{ .Name }} are also supported.
Unknown placeholders are kept and reported as warnings.
"""

import argparse
import re
import sys
from pathlib import Path

PATTERN = re.compile(r"\{\{\s*\.?([A-Za-z_][A-Za-z0-9_]*)\s*\}\}")


def parse_vars(values: list[str]) -> dict[str, str]:
    variables: dict[str, str] = {}
    for item in values:
        if "=" not in item:
            raise ValueError(f"invalid --var (expected key=value): {item}")
        key, value = item.split("=", 1)
        variables[key.strip()] = value
    return variables


def render(content: str, variables: dict[str, str]) -> tuple[str, set[str]]:
    missing: set[str] = set()

    def replace(match: re.Match[str]) -> str:
        key = match.group(1)
        if key in variables:
            return variables[key]
        missing.add(key)
        return match.group(0)

    return PATTERN.sub(replace, content), missing


def main() -> int:
    parser = argparse.ArgumentParser(description="Render a vibebiz template")
    parser.add_argument("template", help="template path")
    parser.add_argument("output", help="output path")
    parser.add_argument("--var", action="append", default=[], help="key=value")
    parser.add_argument("--force", action="store_true", help="overwrite existing output")
    args = parser.parse_args()

    template = Path(args.template)
    output = Path(args.output)
    if not template.is_file():
        print(f"template not found: {template}")
        return 1
    if output.exists() and not args.force:
        print(f"output exists (use --force): {output}")
        return 1

    try:
        variables = parse_vars(args.var)
    except ValueError as exc:
        print(exc)
        return 1

    content, missing = render(template.read_text(encoding="utf-8"), variables)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(content, encoding="utf-8")

    print(f"generated: {output}")
    if missing:
        print(f"warning: unresolved placeholders: {', '.join(sorted(missing))}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
