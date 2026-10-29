# Chat Transcript Local JSONL Implementation Plan

**Status:** Implemented

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Persist chat transcript to `<workspace>/.chathistory/session.jsonl`, keep sandbox turns running after SSE disconnect, and load history through CP.

**Architecture:** sandbox is the only JSONL writer. `ChatTurnManager` owns background turns and SSE subscribers. CP/sandbox exposes a read-only history API. Web loads history before sending an initial prompt.

**Tech Stack:** Python 3.13 + FastAPI, Go 1.24 + Echo, Vue 3 + Pinia, pytest, Go testing, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-29-chat-transcript-local-jsonl-design.md`

## Global Constraints

- JSONL is the only transcript source of truth.
- No MySQL/Redis chat writes in this phase.
- `.chathistory` is hidden from file tree, code view, preview, and ordinary file APIs.
- One active turn per project; disconnect cancels only the SSE subscription.
- Explicit stop and timeout write `status=stopped`.

---

### Task 1: JSONL writer

**Files:**
- Create: `packages/sandbox/app/services/chat_history.py`
- Test: `packages/sandbox/tests/test_chat_history.py`

- [x] Write `ChatHistory.append_user`, `append_assistant`, `read_tail`, `_last_message_id`.
- [x] Append header on first write, use `O_APPEND`, `fsync`, `0700/0600`.
- [x] Ensure `.chathistory/` is added to workspace `.gitignore`.

### Task 2: Background turn manager

**Files:**
- Create: `packages/sandbox/app/services/chat_turns.py`
- Modify: `packages/sandbox/app/main.py`
- Modify: `packages/sandbox/app/config.py`
- Test: `packages/sandbox/tests/test_chat_history.py`

- [x] Start a turn, append user entry, launch `asyncio.Task`.
- [x] Broadcast Agent events to SSE subscriber queues.
- [x] Continue after subscriber disconnect.
- [x] Append assistant on completion, error, stop, or timeout.
- [x] Enforce one active turn per project and 409 conflict.

### Task 3: Sandbox routes and isolation

**Files:**
- Modify: `packages/sandbox/app/routers/chat.py`
- Modify: `packages/sandbox/app/services/workspace.py`
- Test: `packages/sandbox/tests/test_api.py`
- Test: `packages/sandbox/tests/test_workspace.py`

- [x] Add `GET /internal/projects/:id/chat/history?limit=`.
- [x] Add `POST /internal/projects/:id/chat/stop`.
- [x] Add `.chathistory` to `IGNORED_PARTS`.
- [x] Verify history is hidden from files and preview.

### Task 4: CP history API

**Files:**
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`
- Create: `packages/backend/internal/handler/chat_history.go`
- Create: `packages/backend/internal/handler/chat_history_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/cmd/cp/main.go`

- [x] Add `ReadChatHistory` to sandbox client and router.
- [x] Add `GET /cp/api/v1/projects/:id/chat/history`.
- [x] Validate session, workspace membership, limit, and JSONL.
- [x] Skip header, unknown types, and bad lines.

### Task 5: DP stop API

**Files:**
- Create: `packages/backend/internal/handler/chat_stop.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`

- [x] Add `POST /dp/api/v1/projects/:id/chat/stop`.
- [x] Forward to sandbox stop endpoint.

### Task 6: Web history load and disconnect semantics

**Files:**
- Modify: `packages/web/src/api/chat.ts`
- Modify: `packages/web/src/stores/chat.ts`
- Modify: `packages/web/src/stores/chat.test.ts`
- Modify: `packages/web/src/views/EditorView.vue`
- Modify: `packages/web/src/components/editor/ChatPanel.vue`
- Modify: `packages/web/src/stores/files.ts`
- Modify: `packages/web/src/types/index.ts`

- [x] Fetch history from CP and map to `ChatMessage`.
- [x] Do not resend initial prompt when history exists.
- [x] Page unmount aborts local SSE only; stop button calls DP stop API.
- [x] Filter `.chathistory` from the file tree.

### Verification

- [x] `cd packages/sandbox && .venv/bin/python -m pytest -q`
- [x] `cd packages/backend && go test ./...`
- [x] `cd packages/web && npm test && npm run build`
- [x] `make quality`
- [x] `make validate`
