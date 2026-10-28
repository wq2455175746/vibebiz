#!/usr/bin/env python3
"""Language-level quality gate for the vibebiz harness.

- Go: gofmt + go vet + forbidden output patterns
- Python: ruff (if installed) + forbidden patterns
- TypeScript/Vue: eslint (if installed) + forbidden patterns + palette
Exit code: 0 = pass (warnings allowed), 1 = issues.
"""

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SKIP_DIRS = {"node_modules", "dist", ".venv", "__pycache__", ".pytest_cache", ".git"}

ALLOWED_HEX = {
    "#000",
    "#000000",
    "#fff",
    "#ffffff",
    "#111111",
    "#1f2937",
    "#6b7280",
    "#e5e7eb",
    "#d1d5db",
    "#f5f6f8",
    "#f3f4f6",
    "#2563eb",
    "#eff4ff",
}

ISSUES: list[str] = []


def issue(message: str) -> None:
    ISSUES.append(message)
    print(f"  ✗ {message}")


def ok(message: str) -> None:
    print(f"  ✓ {message}")


def run(cmd: list[str], cwd: Path, label: str) -> bool:
    try:
        result = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True, timeout=180)
    except FileNotFoundError:
        print(f"  ⚠ {label} skipped: {cmd[0]} not found")
        return True
    except subprocess.TimeoutExpired:
        issue(f"{label} timed out")
        return False

    if result.returncode != 0:
        detail = (result.stdout + result.stderr).strip()
        issue(f"{label} failed:\n{detail[:1200]}")
        return False
    ok(label)
    return True


def scan(relative: str, suffixes: tuple[str, ...], pattern: str, label: str) -> None:
    base = ROOT / relative
    if not base.exists():
        return
    regex = re.compile(pattern)
    hits = 0
    for path in base.rglob("*"):
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        if not path.is_file() or path.suffix not in suffixes:
            continue
        for number, line in enumerate(path.read_text(encoding="utf-8", errors="ignore").splitlines(), 1):
            if regex.search(line):
                issue(f"{path.relative_to(ROOT)}:{number}: {label}")
                hits += 1
    if hits == 0:
        ok(f"{relative}: no {label}")


def check_go() -> None:
    print("\n[Go]")
    backend = ROOT / "packages/backend"
    try:
        result = subprocess.run(["gofmt", "-l", "."], cwd=backend, capture_output=True, text=True)
        files = [line for line in result.stdout.splitlines() if line.strip()]
        if files:
            issue(f"gofmt needed: {', '.join(files)}")
        else:
            ok("gofmt")
    except FileNotFoundError:
        print("  ⚠ gofmt skipped: not found")
    run(["go", "vet", "./..."], backend, "go vet")
    scan("packages/backend", (".go",), r"\b(fmt|log)\.Print", "fmt/log Print")
    scan("packages/backend", (".go",), r"\b(TODO|FIXME|HACK)\b(?!\s*\()", "bare TODO/FIXME")


def check_python() -> None:
    print("\n[Python]")
    sandbox = ROOT / "packages/sandbox"
    python = sandbox / ".venv/bin/python"
    if python.exists():
        run([str(python), "-m", "ruff", "check", "app", "tests"], sandbox, "ruff check")
        run([str(python), "-m", "ruff", "format", "--check", "app", "tests"], sandbox, "ruff format")
    else:
        print("  ⚠ ruff skipped: packages/sandbox/.venv missing")
    scan("packages/sandbox/app", (".py",), r"^\s*print\(", "print()")
    scan("packages/sandbox/app", (".py",), r"except\s*:", "bare except")
    scan("packages/sandbox/app", (".py",), r"\b(TODO|FIXME|HACK)\b(?!\s*\()", "bare TODO/FIXME")


FRONTENDS = ["packages/web", "packages/ops-web"]


def check_typescript() -> None:
    print("\n[TypeScript/Vue]")
    for relative in FRONTENDS:
        frontend = ROOT / relative
        eslint = frontend / "node_modules/.bin/eslint"
        if eslint.exists():
            run([str(eslint), "src", "--max-warnings=0"], frontend, f"{relative} eslint")
        else:
            print(f"  ⚠ eslint skipped: {relative}/node_modules missing")
        scan(f"{relative}/src", (".ts", ".vue"), r"console\.log\(", "console.log")
        scan(f"{relative}/src", (".ts", ".vue"), r"@ts-(ignore|nocheck)", "@ts-ignore")
        scan(f"{relative}/src", (".ts", ".vue"), r"\b(TODO|FIXME|HACK)\b(?!\s*\()", "bare TODO/FIXME")
    check_palette()


def check_palette() -> None:
    regex = re.compile(r"#[0-9a-fA-F]{3,8}\b")
    hits = 0
    for relative in FRONTENDS:
        base = ROOT / relative / "src"
        for path in base.rglob("*"):
            if any(part in SKIP_DIRS for part in path.parts):
                continue
            if not path.is_file() or path.suffix not in {".vue", ".css", ".ts"}:
                continue
            for number, line in enumerate(
                path.read_text(encoding="utf-8", errors="ignore").splitlines(), 1
            ):
                for color in regex.findall(line):
                    if color.lower() not in ALLOWED_HEX:
                        issue(f"{path.relative_to(ROOT)}:{number}: color outside palette: {color}")
                        hits += 1
    if hits == 0:
        ok("palette (black/white/gray/blue)")


def main() -> int:
    print("=" * 60)
    print("  Vibebiz Code Quality")
    print("=" * 60)
    check_go()
    check_python()
    check_typescript()
    print("\n" + "=" * 60)
    if ISSUES:
        print(f"  {len(ISSUES)} issue(s)")
        return 1
    print("  all quality checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
