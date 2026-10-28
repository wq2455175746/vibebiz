#!/usr/bin/env python3
"""Maintain the .ai memory workspace.

Usage:
  python3 scripts/update_memory.py --checkpoint
  python3 scripts/update_memory.py --add
  python3 scripts/update_memory.py --failure "message" --service backend
"""

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def get_current_focus() -> dict:
    focus_file = ROOT / ".ai/CURRENT_FOCUS"
    if not focus_file.exists():
        return {}
    return json.loads(focus_file.read_text(encoding="utf-8"))


def save_checkpoint() -> None:
    focus = get_current_focus()
    if not focus:
        print("✗ No CURRENT_FOCUS found. Run 'make init' first.")
        return

    task_id = focus.get("task_id", "unknown")
    checkpoint_dir = ROOT / ".ai/checkpoints"
    checkpoint_dir.mkdir(parents=True, exist_ok=True)

    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    checkpoint_file = checkpoint_dir / f"task-{task_id}-{timestamp}.json"
    payload = {
        "timestamp": datetime.now().isoformat(),
        "reason": "manual",
        "focus": focus,
        "session_end": True,
    }
    checkpoint_file.write_text(
        json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    print(f"✓ Checkpoint saved: {checkpoint_file.relative_to(ROOT)}")


def add_lesson(lesson_type: str, title: str, context: str, action: str, tags: list[str]) -> None:
    lessons_file = ROOT / ".ai/memory/lessons.json"
    try:
        data = json.loads(lessons_file.read_text(encoding="utf-8"))
    except (FileNotFoundError, json.JSONDecodeError):
        data = {"lessons": []}

    max_num = 0
    for lesson in data.get("lessons", []):
        lesson_id = lesson.get("id", "")
        if lesson_id.startswith("L"):
            try:
                max_num = max(max_num, int(lesson_id[1:]))
            except ValueError:
                pass

    lesson = {
        "id": f"L{max_num + 1:03d}",
        "date": datetime.now().strftime("%Y-%m-%d"),
        "type": lesson_type,
        "title": title,
        "context": context,
        "action": action,
        "tags": tags,
    }
    data.setdefault("lessons", []).append(lesson)
    lessons_file.write_text(
        json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    print(f"✓ Lesson {lesson['id']} added: {title}")


def add_failure(service: str, message: str, severity: str = "ERROR") -> None:
    focus = get_current_focus()
    task_id = focus.get("task_id", "unknown")
    log_file = ROOT / ".ai/traces/failures.log"
    log_file.parent.mkdir(parents=True, exist_ok=True)
    timestamp = datetime.now().strftime("%Y-%m-%dT%H:%M:%S")
    entry = f"[{timestamp}] [{severity}] [{service}] [{task_id}] {message}\n"
    with log_file.open("a", encoding="utf-8") as handle:
        handle.write(entry)
    print(f"✓ Failure logged: {message[:80]}")


def interactive_add() -> None:
    print("=== Add New Lesson ===")
    types = ["architecture", "pattern", "config", "design", "bugfix", "deployment", "testing", "security"]
    for index, name in enumerate(types):
        print(f"  [{index}] {name}")
    raw_type = input("Select type [0]: ").strip()
    try:
        lesson_type = types[int(raw_type)] if raw_type else types[0]
    except (ValueError, IndexError):
        lesson_type = types[0]

    title = input("Title: ").strip()
    if not title:
        print("✗ Title is required")
        return
    context = input("Context: ").strip()
    action = input("Action: ").strip()
    tags_input = input("Tags (comma-separated): ").strip()
    tags = [tag.strip() for tag in tags_input.split(",") if tag.strip()] if tags_input else []
    add_lesson(lesson_type, title, context, action, tags)


def main() -> int:
    parser = argparse.ArgumentParser(description="Vibebiz harness memory updater")
    parser.add_argument("--checkpoint", action="store_true", help="save CURRENT_FOCUS checkpoint")
    parser.add_argument("--add", action="store_true", help="interactively add a lesson")
    parser.add_argument("--failure", metavar="MESSAGE", help="append a failure to traces")
    parser.add_argument("--service", default="unknown", help="service name for failure logging")
    args = parser.parse_args()

    if args.checkpoint:
        save_checkpoint()
    elif args.add:
        interactive_add()
    elif args.failure:
        add_failure(args.service, args.failure)
    else:
        parser.print_help()
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
