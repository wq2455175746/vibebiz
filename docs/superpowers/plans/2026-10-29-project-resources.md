# Project Resources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add project-scoped knowledge files uploaded to S3, adopted into `.resources`, referenceable by `@` in chat.

**Architecture:** Web uploads directly to S3 via DP presign. Project creation happens after upload. DP adopts staging objects into the project key, downloads them to sandbox, and tracks metadata in MySQL `project_resource`. Sandbox owns `.resources` and injects referenced file paths into the Agent prompt.

**Tech Stack:** Go 1.24 + Echo + GORM + MySQL, Vue 3 + Pinia + Monaco, Python 3.13 + FastAPI, AWS SDK S3.

**Spec:** `docs/superpowers/specs/2026-10-29-project-resources-design.md`

## Global Constraints

- Single file 20 MB; project total 200 MB; max 10 references per message.
- Allowed: pdf/doc/docx/xls/xlsx/ppt/pptx; text/markdown/html/css/js/ts/json/yaml/xml/csv; png/jpg/jpeg/webp/gif.
- `.resources` must not appear in file tree, normal file API, preview, or git.
- S3 storage name is `<resourceId><ext>`; Chinese original name stays in MySQL/manifest.
- CP does not carry resource bytes.
- Do not log file bodies, S3 keys? Keys are not secrets, but do not log presigned URLs or credentials.
- Run `make test`, `make quality`, `make validate` before completion.

---

### Task 1: MySQL resource metadata

**Files:**
- Create: `packages/backend/migrations/00011_project_resource.sql`
- Create: `packages/backend/internal/dao/model/resource.go`
- Create: `packages/backend/internal/dao/resource_repository.go`
- Create: `packages/backend/internal/dao/resource_repository_test.go`
- Modify: `packages/backend/internal/dao/repository.go`
- Modify: `packages/backend/migrations/migrations_test.go`

**Interfaces:**
- Produces: `dao.ProjectResourceRepository` with `Create`, `GetByResourceID`, `ListReadyByProject`, `SumReadyByProject`, `UpdateStatus`, `SetProject`, `Delete`.
- Produces: `dao.ProjectResourceRecord` fields matching spec table.

- [ ] **Step 1:** Write migration with `project_resource` table and indexes from spec.
- [ ] **Step 2:** Add GORM model and repository methods.
- [ ] **Step 3:** Add sqlmock tests for create, lookup, sum, status transitions.
- [ ] **Step 4:** Run `go test ./internal/dao/...`.
- [ ] **Step 5:** Commit.

### Task 2: S3 copy/get/delete

**Files:**
- Modify: `packages/backend/internal/s3/presigner.go`
- Modify: `packages/backend/internal/s3/presigner_test.go`

**Interfaces:**
- Produces: `Copy(ctx, srcKey, dstKey, contentType string) error`
- Produces: `Get(ctx, key string) (io.ReadCloser, int64, error)`
- Produces: `Delete(ctx, key string) error`

- [ ] **Step 1:** Add failing httptest tests for Copy/Get/Delete.
- [ ] **Step 2:** Implement SDK calls.
- [ ] **Step 3:** Run `go test ./internal/s3/...`.
- [ ] **Step 4:** Commit.

### Task 3: DP resource service and API

**Files:**
- Create: `packages/backend/internal/resource/service.go`
- Create: `packages/backend/internal/resource/service_test.go`
- Create: `packages/backend/internal/handler/resource.go`
- Create: `packages/backend/internal/handler/resource_test.go`
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/cmd/dp/main.go`

**Interfaces:**
- Consumes: `dao.ProjectResourceRepository`, `s3.Presigner`, `client.SandboxResources`.
- Produces: DP routes from spec.
- Produces: sandbox client methods `ListResources`, `WriteResource`, `DeleteResource`.

- [ ] **Step 1:** Write failing service tests for filename sanitization, allowed types, limits, adopt state transitions.
- [ ] **Step 2:** Implement resource service.
- [ ] **Step 3:** Add sandbox client methods and resource handler/routes.
- [ ] **Step 4:** Wire S3 and repository into `cmd/dp`.
- [ ] **Step 5:** Run `go test ./...`.
- [ ] **Step 6:** Commit.

### Task 4: Sandbox resource storage

**Files:**
- Create: `packages/sandbox/app/services/resources.py`
- Create: `packages/sandbox/app/routers/resources.py`
- Create: `packages/sandbox/tests/test_resources.py`
- Modify: `packages/sandbox/app/main.py`
- Modify: `packages/sandbox/app/services/workspace.py`
- Modify: `packages/sandbox/app/services/versioning.py`

**Interfaces:**
- Produces: internal list/write/delete resource endpoints.
- Produces: `.resources/manifest.json` with fields from spec.
- Produces: `.resources` excluded from file list, read, preview, git.

- [ ] **Step 1:** Write failing tests for path traversal, duplicate display names, size limits, manifest atomic update, isolation.
- [ ] **Step 2:** Implement resource service and router.
- [ ] **Step 3:** Add `.resources` to ignore sets and gitignore.
- [ ] **Step 4:** Run `pytest`.
- [ ] **Step 5:** Commit.

### Task 5: Chat references and Agent prompt

**Files:**
- Modify: `packages/sandbox/app/routers/chat.py`
- Modify: `packages/sandbox/app/services/chat_turns.py`
- Modify: `packages/sandbox/app/services/chat_history.py`
- Modify: `packages/sandbox/app/services/claude_agent.py` only if prompt signature needs it
- Modify: `packages/sandbox/tests/test_api.py`
- Modify: `packages/sandbox/tests/test_chat_history.py`

**Interfaces:**
- Chat request accepts `references: [{kind:"resource", id} | {kind:"project_file", path}]`.
- Sandbox resolves resource IDs via manifest and validates project paths.
- Enriched prompt includes relative paths and document/image instructions.
- JSONL user message stores references.

- [ ] **Step 1:** Write failing tests for valid/invalid refs, max 10, prompt injection, history references.
- [ ] **Step 2:** Implement reference resolution and prompt enrichment.
- [ ] **Step 3:** Persist references in history.
- [ ] **Step 4:** Run `pytest`.
- [ ] **Step 5:** Commit.

### Task 6: CP history references

**Files:**
- Modify: `packages/backend/internal/handler/chat_history.go`
- Modify: `packages/backend/internal/handler/chat_history_test.go`

**Interfaces:**
- `chatHistoryMessage` adds `references` for user messages.
- CP parser returns references unchanged from JSONL.

- [ ] **Step 1:** Add failing parser test with references.
- [ ] **Step 2:** Extend parser.
- [ ] **Step 3:** Run `go test ./internal/handler/...`.
- [ ] **Step 4:** Commit.

### Task 7: Web upload, `@`, chips, history

**Files:**
- Create: `packages/web/src/api/resources.ts`
- Create: `packages/web/src/stores/resources.ts`
- Create: `packages/web/src/components/editor/AttachmentPicker.vue`
- Create: `packages/web/src/components/editor/ReferenceChips.vue`
- Create: `packages/web/src/components/editor/MentionMenu.vue`
- Create: `packages/web/src/utils/mentions.ts`
- Create: `packages/web/src/utils/mentions.test.ts`
- Create: `packages/web/src/api/resources.test.ts`
- Modify: `packages/web/src/api/chat.ts`
- Modify: `packages/web/src/stores/chat.ts`
- Modify: `packages/web/src/components/home/HomeHero.vue`
- Modify: `packages/web/src/views/HomeView.vue`
- Modify: `packages/web/src/components/editor/ChatPanel.vue`
- Modify: `packages/web/src/components/editor/ChatMessage.vue`
- Modify: `packages/web/src/types/index.ts`
- Modify: `packages/web/src/styles/editor.css`
- Modify: `packages/web/src/styles/home.css`

**Interfaces:**
- Resource store: load, upload, remove, project resources.
- Home: File[] memory, upload after project creation order.
- Chat: send references; @ parser maps display names/project paths.
- History chips render resource/project-file references.

- [ ] **Step 1:** Write failing tests for mention parsing, upload order, reference dedupe, API calls.
- [ ] **Step 2:** Implement API/store/utils.
- [ ] **Step 3:** Implement picker, chips, mention menu and wire home/editor.
- [ ] **Step 4:** Add CSS.
- [ ] **Step 5:** Run `npm test`, `npm run lint`, `npm run build`.
- [ ] **Step 6:** Commit.

### Task 8: Integration and migration

- [ ] Apply `00011_project_resource.sql`.
- [ ] Restart CP/DP/sandbox/web.
- [ ] E2E: home file upload before project, adopt after create, `@` in editor, history chips.
- [ ] Verify PDF skill path or explicit failure message.
- [ ] Run `make test`, `make quality`, `make validate`.
- [ ] `make checkpoint`.
