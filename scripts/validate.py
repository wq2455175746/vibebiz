#!/usr/bin/env python3
"""Static validation for the vibebiz harness.

Checks structure, AI workspace files, contracts and dependency direction.
Exit code: 0 = no errors (warnings allowed), 1 = errors.
"""

import json
import sys
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

ROOT_FILES = [
    "README.md",
    "AGENTS.md",
    "Makefile",
    ".gitignore",
    "docker-compose.yml",
]

AI_FILES = [
    ".ai/AGENTS.md",
    ".ai/CURRENT_FOCUS",
    ".ai/TODO.md",
    ".ai/rules/architecture.md",
    ".ai/memory/lessons.json",
    ".ai/memory/anti-patterns.md",
    ".ai/memory/successful-patterns.md",
    ".ai/traces/failures.log",
]

RULE_FILES = [
    ".ai/rules/go.md",
    ".ai/rules/python.md",
    ".ai/rules/typescript.md",
]

TEMPLATE_FILES = [
    "templates/README.md",
    "templates/architecture-schema.json",
    "templates/go-handler.go.j2",
    "templates/go-client.go.j2",
    "templates/python-router.py.j2",
    "templates/python-service.py.j2",
    "templates/python-script.py.j2",
    "templates/python-test.py.j2",
    "templates/typescript-api.ts.j2",
    "templates/typescript-store.ts.j2",
    "templates/typescript-util.ts.j2",
    "templates/typescript-util.test.ts.j2",
    "templates/vue-view.vue.j2",
    "templates/vue-component.vue.j2",
    "templates/design-doc.md",
    "templates/exec-plan.md",
    "templates/current-focus.json",
    "templates/checkpoint.json",
    "templates/package-AGENTS.md",
]

TOOLING_FILES = [
    ".editorconfig",
    "scripts/check_quality.py",
    "scripts/generate_from_template.py",
    "packages/sandbox/pyproject.toml",
    "packages/web/eslint.config.js",
    "packages/web/.prettierrc.json",
    "packages/ops-web/eslint.config.js",
    "packages/ops-web/.prettierrc.json",
]

PACKAGES = {
    "packages/backend": {
        "required_files": [
            "go.mod",
            "Dockerfile",
            "Makefile",
            ".env.example",
            ".dockerignore",
            ".ai/AGENTS.md",
            "cmd/cp/main.go",
            "cmd/dp/main.go",
            "cmd/migrate/main.go",
            "migrations/00005_default_users.sql",
            "migrations/00006_settings.sql",
            "internal/account/service.go",
            "internal/s3/presigner.go",
            "internal/handler/settings.go",
            "internal/handler/storage.go",
        ],
        "required_dirs": [
            "internal/admin",
            "internal/account",
            "internal/auth",
            "internal/client",
            "internal/config",
            "internal/handler",
            "internal/model",
            "internal/dao",
            "internal/dao/model",
            "migrations",
            "internal/observability",
            "internal/response",
            "internal/router",
            "internal/server",
            "internal/store",
        ],
    },
    "packages/sandbox": {
        "required_files": [
            "requirements.txt",
            "requirements-dev.txt",
            "Dockerfile",
            "Makefile",
            ".env.example",
            ".dockerignore",
            "pytest.ini",
            ".ai/AGENTS.md",
            "app/main.py",
        ],
        "required_dirs": ["app/routers", "app/services", "tests"],
    },
    "packages/web": {
        "required_files": [
            "package.json",
            "index.html",
            "Dockerfile",
            "Makefile",
            ".env.example",
            ".dockerignore",
            "nginx.conf",
            ".ai/AGENTS.md",
            "src/main.ts",
            "src/api/auth.ts",
            "src/stores/auth.ts",
            "src/views/LoginView.vue",
            "src/layouts/SettingsLayout.vue",
            "src/views/settings/ProfileView.vue",
            "src/views/settings/StorageView.vue",
            "src/views/settings/WorkspaceView.vue",
            "src/views/settings/MembersView.vue",
        ],
        "required_dirs": ["src/api", "src/stores", "src/views", "src/components", "src/utils", "src/layouts"],
    },
    "packages/ops-web": {
        "required_files": [
            "package.json",
            "index.html",
            "Dockerfile",
            "Makefile",
            ".env.example",
            ".dockerignore",
            "nginx.conf",
            ".ai/AGENTS.md",
            "src/main.ts",
            "src/api/auth.ts",
            "src/stores/auth.ts",
            "src/views/LoginView.vue",
        ],
        "required_dirs": [
            "src/api",
            "src/components",
            "src/layouts",
            "src/router",
            "src/stores",
            "src/styles",
            "src/views",
        ],
    },
}

ERRORS = []
WARNINGS = []


def check(message: str) -> None:
    print(f"  ✓ {message}")


def warn(message: str) -> None:
    WARNINGS.append(message)
    print(f"  ⚠ {message}")


def error(message: str) -> None:
    ERRORS.append(message)
    print(f"  ✗ {message}")


def read(relative: str) -> str:
    path = ROOT / relative
    if not path.exists():
        return ""
    return path.read_text(encoding="utf-8")


def validate_root() -> None:
    print("\n[1/7] Root files")
    for name in ROOT_FILES:
        if (ROOT / name).exists():
            check(name)
        else:
            error(f"missing root file: {name}")
    if (ROOT / ".env.example").exists():
        error("root .env.example is forbidden; keep .env.example per package")
    else:
        check("no root .env.example")


def validate_ai() -> None:
    print("\n[2/7] AI workspace (.ai/)")
    for name in AI_FILES:
        if (ROOT / name).exists():
            check(name)
        else:
            error(f"missing AI file: {name}")

    focus_path = ROOT / ".ai/CURRENT_FOCUS"
    if focus_path.exists():
        try:
            focus = json.loads(focus_path.read_text(encoding="utf-8"))
            for key in ("service", "task_id", "status", "updated_at"):
                if key not in focus:
                    error(f"CURRENT_FOCUS missing key: {key}")
            if focus.get("service") not in [
                "all",
                "web",
                "ops-web",
                "backend",
                "cp",
                "dp",
                "sandbox",
                None,
            ]:
                warn(f"CURRENT_FOCUS.service is unknown: {focus.get('service')}")
            if focus.get("updated_at"):
                try:
                    updated = datetime.fromisoformat(focus["updated_at"])
                    age = (datetime.now(updated.tzinfo) - updated).days
                    if age > 30:
                        warn(f"CURRENT_FOCUS is {age} days old; update the task focus")
                except ValueError:
                    warn("CURRENT_FOCUS.updated_at is not a valid ISO timestamp")
        except json.JSONDecodeError as exc:
            error(f"CURRENT_FOCUS is invalid JSON: {exc}")

    lessons_path = ROOT / ".ai/memory/lessons.json"
    if lessons_path.exists():
        try:
            data = json.loads(lessons_path.read_text(encoding="utf-8"))
            if not isinstance(data.get("lessons"), list):
                error("lessons.json must contain a 'lessons' array")
            else:
                for lesson in data["lessons"]:
                    for key in ("id", "date", "title", "action"):
                        if key not in lesson:
                            error(f"lesson {lesson.get('id', '?')} missing key: {key}")
        except json.JSONDecodeError as exc:
            error(f"lessons.json is invalid JSON: {exc}")

    for name in ("anti-patterns.md", "successful-patterns.md"):
        path = ROOT / ".ai/memory" / name
        if path.exists() and not path.read_text(encoding="utf-8").strip():
            warn(f".ai/memory/{name} is empty")


def validate_templates_and_rules() -> None:
    print("\n[3/7] Templates & rules")
    for name in RULE_FILES:
        path = ROOT / name
        if not path.exists():
            error(f"missing rule file: {name}")
        elif not path.read_text(encoding="utf-8").strip():
            error(f"empty rule file: {name}")
        else:
            check(name)
    for name in TEMPLATE_FILES:
        path = ROOT / name
        if not path.exists():
            error(f"missing template: {name}")
        elif path.suffix in {".j2", ".md", ".json"} and not path.read_text(encoding="utf-8").strip():
            error(f"empty template: {name}")
        elif path.suffix == ".json":
            try:
                json.loads(path.read_text(encoding="utf-8"))
                check(name)
            except json.JSONDecodeError as exc:
                error(f"invalid JSON template {name}: {exc}")
        else:
            check(name)
    for name in TOOLING_FILES:
        if (ROOT / name).exists():
            check(name)
        else:
            error(f"missing tooling file: {name}")


def validate_packages() -> None:
    print("\n[4/7] Packages")
    for package, rules in PACKAGES.items():
        package_dir = ROOT / package
        if not package_dir.is_dir():
            error(f"missing package directory: {package}")
            continue
        for name in rules["required_files"]:
            if not (package_dir / name).exists():
                error(f"{package}: missing file '{name}'")
        for name in rules["required_dirs"]:
            if not (package_dir / name).is_dir():
                error(f"{package}: missing directory '{name}'")
        check(f"{package} structure")


def require_absent(relative: str, needle: str) -> None:
    content = read(relative)
    if not content:
        error(f"cannot check contract: {relative} missing or empty")
        return
    if needle in content:
        error(f"{relative} contains forbidden contract fragment: {needle!r}")
    else:
        check(f"{relative} absent {needle!r}")


def require_regex_absent(relative: str, pattern: str) -> None:
    import re

    content = read(relative)
    if not content:
        error(f"cannot check contract: {relative} missing or empty")
        return
    if re.search(pattern, content):
        error(f"{relative} matches forbidden pattern: {pattern!r}")
    else:
        check(f"{relative} absent pattern {pattern!r}")


def require_contains(relative: str, needles: list[str]) -> None:
    content = read(relative)
    if not content:
        error(f"cannot check contract: {relative} missing or empty")
        return
    missing = False
    for needle in needles:
        if needle not in content:
            error(f"{relative} missing contract fragment: {needle!r}")
            missing = True
    if not missing:
        check(f"{relative} contract")


def validate_contracts() -> None:
    print("\n[5/7] Contracts")
    require_contains(
        "Makefile",
        [
            "validate:",
            "test:",
            "dev:",
            "docker-build:",
            "checkpoint:",
            "memory-add:",
            "install-ops-web:",
            "dev-cp:",
            "dev-dp:",
            "dev-ops-web:",
            "test-ops-web:",
        ],
    )
    require_contains("docker-compose.yml", ["sandbox:", "cp:", "dp:", "web:", "ops-web:", "SERVICE: cp", "SERVICE: dp"])
    require_contains(
        "scripts/dev.sh",
        ["packages/backend", "packages/sandbox", "packages/web", "packages/ops-web", ".env.example", "source ./.env"],
    )
    require_contains(
        "packages/backend/Makefile",
        ["dev-cp:", "dev-dp:", "test:", "quality:", "build:", "install:", "db-create:", "db-migrate-up:", "db-migrate-status:"],
    )
    require_contains("packages/backend/.env.example", [
        "HOST=", "CP_PORT=", "DP_PORT=", "SANDBOX_BASE_URL=", "CORS_ORIGINS=",
        "MYSQL_HOST=", "MYSQL_DATABASE=", "REDIS_HOST=", "S3_BUCKET=", "S3_ENDPOINT=",
    ])
    for name in (
        "00001_identity_workspace.sql",
        "00002_project_resource.sql",
        "00003_ai_chat_rag.sql",
        "00004_seed_topics_roles.sql",
    ):
        require_contains(f"packages/backend/migrations/{name}", ["-- +goose Up", "-- +goose Down"])
    require_contains("packages/backend/cmd/migrate/main.go", ["goose.Up", "goose.Down", "goose.Status", "goose.GetDBVersion"])
    require_contains("packages/sandbox/Makefile", ["dev:", "test:", "quality:", "install:"])
    require_contains("packages/sandbox/.env.example", ["WORKSPACES_DIR=", "AGENT_MODE=", "LLM_API_KEY="])
    for package in ("web", "ops-web"):
        require_contains(f"packages/{package}/Makefile", ["dev:", "test:", "build:", "quality:", "install:"])
    require_contains("packages/web/.env.example", ["VITE_CP_BASE=", "VITE_DP_BASE=", "VITE_CP_PROXY_TARGET=", "VITE_DP_PROXY_TARGET="])
    require_contains("packages/ops-web/.env.example", ["VITE_OPS_API_BASE=", "VITE_CP_PROXY_TARGET="])
    for package in ("backend", "sandbox", "web", "ops-web"):
        require_contains(f"packages/{package}/.dockerignore", [".env"])

    legacy_server = ROOT / "packages/backend/cmd/server"
    if legacy_server.exists():
        error("packages/backend/cmd/server must not exist; use cmd/cp and cmd/dp")
    else:
        check("no legacy packages/backend/cmd/server")

    control = "packages/backend/internal/router/control.go"
    data = "packages/backend/internal/router/data.go"
    require_contains(control, ["/cp/api/v1", "/admin"])
    require_absent(control, "preview")
    require_absent(control, "/dp/api/v1")
    require_regex_absent(control, r'Group\("/api/v1"')
    require_regex_absent(control, r'\.(GET|POST|PUT|DELETE|PATCH|Any)\(\s*"/api/v1')
    require_contains(data, ["/dp/api/v1", "chat", "preview"])
    require_absent(data, "admin")
    require_absent(data, "/cp/api/v1")
    require_regex_absent(data, r'Group\("/api/v1"')
    require_regex_absent(data, r'\.(GET|POST|PUT|DELETE|PATCH|Any)\(\s*"/api/v1')
    require_contains("packages/backend/cmd/cp/main.go", ["RegisterControl"])
    require_contains("packages/backend/cmd/dp/main.go", ["RegisterData"])
    require_contains("packages/backend/internal/server/server.go", ["/metrics"])
    require_contains("packages/sandbox/app/main.py", ["configure_observability"])
    require_contains("packages/sandbox/app/observability.py", ["/metrics"])
    require_contains("packages/web/src/api/client.ts", ["/cp/api/v1", "/dp/api/v1"])
    require_contains("packages/ops-web/src/api/client.ts", ["/cp/api/v1/admin"])

    require_contains("packages/backend/internal/handler/chat.go", ["Flush()"])
    require_contains("packages/backend/internal/handler/preview.go", ["FlushInterval: -1"])
    require_contains("packages/sandbox/app/services/workspace.py", ["is_relative_to", "sha256"])
    require_contains("packages/sandbox/app/services/events.py", ["[DONE]"])
    require_contains("packages/sandbox/app/routers/chat.py", ["StreamingResponse"])
    require_contains("packages/web/src/utils/sse.ts", ["splitSSE"])
    require_contains("packages/web/src/api/client.ts", ["unwrapEnvelope", "ApiError"])
    require_contains("packages/web/src/views/EditorView.vue", ["PreviewPanel", "CodePanel"])
    require_contains("packages/web/package.json", ["\"lint\"", "eslint"])
    require_contains("packages/sandbox/pyproject.toml", ["[tool.ruff]", "select"])
    require_contains("templates/architecture-schema.json", ["packages/web", "packages/backend", "packages/sandbox"])


SKIP_DIRS = {"node_modules", "dist", ".venv", "__pycache__", ".pytest_cache", ".git"}


def scan(relative: str, suffixes: tuple[str, ...], needles: list[str]) -> list[str]:
    base = ROOT / relative
    hits = []
    if not base.exists():
        return hits
    for path in base.rglob("*"):
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        if path.is_file() and path.suffix in suffixes:
            try:
                content = path.read_text(encoding="utf-8", errors="ignore")
            except OSError:
                continue
            for needle in needles:
                if needle in content:
                    hits.append(f"{path.relative_to(ROOT)} contains {needle!r}")
    return hits


def validate_dependency_direction() -> None:
    print("\n[6/7] Dependency direction")
    violations = []
    violations += scan(
        "packages/web/src",
        (".ts", ".vue"),
        ["localhost:8000", "127.0.0.1:8000", "sandbox:8000", "/cp/api/v1/admin", "/internal/projects"],
    )
    violations += scan(
        "packages/ops-web/src",
        (".ts", ".vue"),
        ["localhost:8000", "127.0.0.1:8000", "sandbox:8000", "/dp/", "/internal/projects"],
    )
    violations += scan("packages/backend", (".go",), ["packages/sandbox", "sandbox/app"])
    violations += scan("packages/sandbox/app", (".py",), ["packages/backend", "vibebiz/internal"])
    if violations:
        for violation in violations:
            error(violation)
    else:
        check("web → CP/DP, ops-web → CP, CP/DP → sandbox")


def validate_freshness() -> None:
    print("\n[7/7] Freshness")
    lessons_path = ROOT / ".ai/memory/lessons.json"
    if lessons_path.exists():
        try:
            lessons = json.loads(lessons_path.read_text(encoding="utf-8")).get("lessons", [])
            dates = [lesson.get("date", "") for lesson in lessons if lesson.get("date")]
            if not dates:
                warn("lessons.json has no dated lessons")
            else:
                latest = max(dates)
                age = (datetime.now() - datetime.strptime(latest, "%Y-%m-%d")).days
                if age > 60:
                    warn(f"latest lesson is {age} days old ({latest})")
                else:
                    check(f"latest lesson {latest}")
        except (json.JSONDecodeError, ValueError):
            pass


def main() -> int:
    print("=" * 60)
    print("  Vibebiz Harness Validation")
    print("=" * 60)

    validate_root()
    validate_ai()
    validate_templates_and_rules()
    validate_packages()
    validate_contracts()
    validate_dependency_direction()
    validate_freshness()

    print("\n" + "=" * 60)
    print(f"  errors={len(ERRORS)} warnings={len(WARNINGS)}")
    print("=" * 60)
    return 1 if ERRORS else 0


if __name__ == "__main__":
    sys.exit(main())
