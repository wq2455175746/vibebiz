#!/usr/bin/env python3
"""Harness auto-updater.

Called on context compaction / session end (manual or platform hook):
  1. save a checkpoint from .ai/CURRENT_FOCUS;
  2. append a session event to .ai/traces/failures.log;
  3. refresh CURRENT_FOCUS.updated_at;
  4. keep only the latest 3 checkpoints per task_id.

Usage:
  python3 scripts/harness_autoupdate.py --reason=compact
"""

import json
import sys
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MAX_CHECKPOINTS = 3


def read_stdin_json() -> dict:
    try:
        raw = sys.stdin.read()
        return json.loads(raw) if raw.strip() else {}
    except (json.JSONDecodeError, OSError, ValueError):
        return {}


def read_focus() -> dict:
    focus_file = ROOT / ".ai/CURRENT_FOCUS"
    if not focus_file.exists():
        return {}
    try:
        return json.loads(focus_file.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return {}


def write_focus(focus: dict) -> None:
    focus["updated_at"] = datetime.now().isoformat()
    (ROOT / ".ai/CURRENT_FOCUS").write_text(
        json.dumps(focus, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )


def save_checkpoint(focus: dict, reason: str) -> Path:
    task_id = focus.get("task_id", "unknown")
    directory = ROOT / ".ai/checkpoints"
    directory.mkdir(parents=True, exist_ok=True)
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    path = directory / f"task-{task_id}-{timestamp}.json"

    payload = {
        "timestamp": datetime.now().isoformat(),
        "reason": reason,
        "focus": focus,
        "session_end": True,
    }
    hook_input = read_stdin_json()
    if hook_input:
        payload["hook_input"] = hook_input
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return path


def log_event(focus: dict, reason: str) -> None:
    log_file = ROOT / ".ai/traces/failures.log"
    log_file.parent.mkdir(parents=True, exist_ok=True)
    timestamp = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
    entry = (
        f"[{timestamp}] [INFO] [harness] [auto] Session {reason}: "
        f"task={focus.get('task_id', 'unknown')} "
        f"name={focus.get('task_name', 'unknown')} "
        f"status={focus.get('status', 'unknown')}\n"
    )
    with log_file.open("a", encoding="utf-8") as handle:
        handle.write(entry)


def clean_checkpoints(focus: dict) -> int:
    task_id = focus.get("task_id", "unknown")
    directory = ROOT / ".ai/checkpoints"
    checkpoints = sorted(
        directory.glob(f"task-{task_id}-*.json"),
        key=lambda path: path.stat().st_mtime,
        reverse=True,
    )
    removed = 0
    for path in checkpoints[MAX_CHECKPOINTS:]:
        path.unlink()
        removed += 1
    return removed


def main() -> int:
    reason = "compact"
    if len(sys.argv) > 1 and sys.argv[1].startswith("--reason="):
        reason = sys.argv[1].split("=", 1)[1]

    focus = read_focus()
    if not focus:
        print("No CURRENT_FOCUS found, skipping harness auto-update")
        return 0

    checkpoint = save_checkpoint(focus, reason)
    print(f"Checkpoint saved: {checkpoint.relative_to(ROOT)}")
    log_event(focus, reason)
    write_focus(focus)
    print("CURRENT_FOCUS timestamp updated")
    removed = clean_checkpoints(focus)
    if removed:
        print(f"Cleaned {removed} old checkpoint(s) (max {MAX_CHECKPOINTS} kept per task)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
