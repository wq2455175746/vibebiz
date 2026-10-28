#!/usr/bin/env python3
"""Layer dependency checker for the vibebiz monorepo.

Rules:
  1. web may only talk to CP / DP over HTTP/SSE.
  2. ops-web may only talk to CP admin, never DP or sandbox.
  3. CP / DP may only talk to sandbox over HTTP/SSE.
  4. no package imports another package's source.
  5. no LLM SDK allowed in backend or web.
  6. no observability SDK allowed in web or ops-web.

Exit code: 0 = pass, 1 = violation.
"""

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SKIP_DIRS = {"node_modules", "dist", ".venv", "__pycache__", ".pytest_cache", ".git"}

BACKEND_FORBIDDEN_LIBS = ["openai", "anthropic", "claude-agent-sdk", "langchain", "llama-index"]
WEB_FORBIDDEN_LIBS = ["openai", "anthropic", "langchain", "llama-index"]
OBSERVABILITY_FORBIDDEN_LIBS = ["@opentelemetry/", "prom-client", "pino", "winston"]

VIOLATIONS = []


def fail(message: str) -> None:
    VIOLATIONS.append(message)
    print(f"  ✗ {message}")


def ok(message: str) -> None:
    print(f"  ✓ {message}")


def files(relative: str, suffixes: tuple[str, ...]):
    base = ROOT / relative
    if not base.exists():
        return
    for path in base.rglob("*"):
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        if path.is_file() and path.suffix in suffixes:
            yield path


def check_legacy_server() -> None:
    legacy = ROOT / "packages/backend/cmd/server"
    if legacy.exists():
        fail("packages/backend/cmd/server must not exist; use cmd/cp and cmd/dp")
    else:
        ok("no legacy packages/backend/cmd/server")


def check_go_mods() -> None:
    go_mod = ROOT / "packages/backend/go.mod"
    if not go_mod.exists():
        fail("packages/backend/go.mod missing")
        return
    content = go_mod.read_text(encoding="utf-8").lower()
    for lib in BACKEND_FORBIDDEN_LIBS:
        if lib in content:
            fail(f"backend/go.mod contains forbidden AI dependency: {lib}")
    ok("backend go.mod has no AI SDK")


def check_frontend_deps(relative: str, label: str) -> None:
    package_json = ROOT / relative
    if not package_json.exists():
        fail(f"{relative} missing")
        return
    data = json.loads(package_json.read_text(encoding="utf-8"))
    deps = {**data.get("dependencies", {}), **data.get("devDependencies", {})}
    failed = False
    for lib in WEB_FORBIDDEN_LIBS:
        if any(name.startswith(lib) for name in deps):
            fail(f"{label}/package.json contains forbidden AI dependency: {lib}")
            failed = True
    for lib in OBSERVABILITY_FORBIDDEN_LIBS:
        if any(name.startswith(lib) for name in deps):
            fail(f"{label}/package.json contains forbidden observability dependency: {lib}")
            failed = True
    if not failed:
        ok(f"{label} package.json has no AI or observability SDK")


def check_sources() -> None:
    patterns = {
        "packages/backend": ((".go",), [r'"[^"]*packages/sandbox', r'"[^"]*sandbox/app']),
        "packages/sandbox/app": ((".py",), [r"packages/backend", r"vibebiz/internal"]),
        "packages/web/src": (
            (".ts", ".vue"),
            [
                r"localhost:8000",
                r"127\.0\.0\.1:8000",
                r"sandbox:8000",
                r"/cp/api/v1/admin",
                r"/internal/projects",
            ],
        ),
        "packages/ops-web/src": (
            (".ts", ".vue"),
            [
                r"/dp/",
                r"localhost:8000",
                r"127\.0\.0\.1:8000",
                r"sandbox:8000",
                r"/internal/projects",
            ],
        ),
    }
    for relative, (suffixes, regexes) in patterns.items():
        hits = 0
        for path in files(relative, suffixes):
            content = path.read_text(encoding="utf-8", errors="ignore")
            for pattern in regexes:
                if re.search(pattern, content):
                    fail(f"{path.relative_to(ROOT)} matches forbidden pattern: {pattern}")
                    hits += 1
        if hits == 0:
            ok(f"{relative} keeps layer boundary")


def main() -> int:
    print("=" * 60)
    print("  Vibebiz Dependency Check")
    print("=" * 60)
    check_legacy_server()
    check_go_mods()
    check_frontend_deps("packages/web/package.json", "web")
    check_frontend_deps("packages/ops-web/package.json", "ops-web")
    check_sources()
    print("=" * 60)
    if VIOLATIONS:
        print(f"  {len(VIOLATIONS)} violation(s)")
        return 1
    print("  all dependency rules passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
