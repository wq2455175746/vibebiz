# ops-web 个人密码修改 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 ops-web 管理员在系统设置中校验旧密码后修改自己的密码，并踢掉其他已登录会话。

**Architecture:** CP 新增 `PUT /cp/api/v1/admin/auth/password`，复用 ops Session 与 bcrypt；服务层负责旧密码校验、更新 `ops_user.password_hash` 与撤销其他 ops Session。ops-web 系统设置页新增表单，API 走现有 admin 客户端。

**Tech Stack:** Go 1.24 + Echo + GORM + bcrypt + go-redis；Vue 3 + PrimeVue + Pinia + axios + vitest。

**Spec:** `docs/superpowers/specs/2026-11-06-ops-self-password-change-design.md`

## Global Constraints

- 密码规则：8-72 位，bcrypt cost 12。
- 旧密码错误返回 HTTP 400 + code 40002，不能返回 401 触发前端登出。
- 成功后保留当前 ops Session，删除同账号其他 ops Session。
- 不记录密码、hash、Session ID。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 先写失败测试，再实现；最后执行 `go test ./...`、ops-web `test`/`build`/`lint`、`make validate`。

---

### Task 1: 后端改密能力（DAO + auth service + Redis session revoke）

**Files:**
- Modify: `packages/backend/internal/auth/session.go`
- Modify: `packages/backend/internal/auth/service.go`
- Modify: `packages/backend/internal/auth/redis_store.go`
- Modify: `packages/backend/internal/dao/identity_repository.go`
- Test: `packages/backend/internal/auth/service_test.go`
- Test: `packages/backend/internal/auth/redis_store_test.go`
- Test: `packages/backend/internal/dao/identity_repository_test.go`

**Interfaces:**
- Produces:
  - `auth.ErrInvalidOldPassword`
  - `auth.ErrPasswordUnchanged`
  - `auth.SessionOthersRevoker.DeleteOthersByUser(ctx, realm, userID, keepSessionID) error`
  - `auth.Service.ChangeOpsPassword(ctx, session *auth.Session, oldPassword, newPassword string) error`
  - `dao.IdentityRepository.FindOpsByID(ctx, id) (*model.OpsUser, error)`
  - `dao.IdentityRepository.UpdateOpsPassword(ctx, id, hash) error`

- [ ] **Step 1: 写 auth service 失败测试**

在 `service_test.go` 的 `fakeOpsFinder` 增加 `FindOpsByID` / `UpdateOpsPassword` 记录字段；增加测试：旧密码错误返回 `ErrInvalidOldPassword`；账号禁用返回 `ErrAccountDisabled`；新旧相同返回 `ErrPasswordUnchanged`；成功更新 hash 且调用 `DeleteOthersByUser`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/auth -run TestChangeOpsPassword -v`
Expected: FAIL，方法不存在。

- [ ] **Step 3: 实现 auth 改密能力**

扩展 `OpsFinder` 接口与 `Service.ChangeOpsPassword`；在 `session.go` 增加 `SessionOthersRevoker`；`RedisStore` 扫描 `:session:ops:*` 并删除同 `userId` 且 `id != keepSessionID` 的 key。

- [ ] **Step 4: 写 DAO 测试并实现**

用 sqlmock 覆盖 `FindOpsByID` 命中/未命中、`UpdateOpsPassword` 更新与不存在。实现两个方法。

- [ ] **Step 5: 运行后端 auth/dao 测试**

Run: `go test ./internal/auth ./internal/dao`
Expected: PASS。

---

### Task 2: Handler + 路由 + 审计

**Files:**
- Modify: `packages/backend/internal/handler/auth.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Test: `packages/backend/internal/router/admin_test.go`

**Interfaces:**
- Consumes: `auth.Service.ChangeOpsPassword`、`auth.ErrInvalidOldPassword`、`auth.ErrPasswordUnchanged`、`auth.ErrAccountDisabled`。
- Produces: `PUT /cp/api/v1/admin/auth/password`，body `{oldPassword,newPassword}`，成功 `{changed:true}`。

- [ ] **Step 1: 写 router 失败测试**

在 `admin_test.go` 增加：普通用户 Cookie 401；旧密码错误 400 + `40002`；短密码 400；成功 200 且当前 ops Session 保留、同账号其他 ops Session 被删除。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/router -run TestAdminChangesOwnPassword -v`
Expected: FAIL，404 / OpsFinder 未配置。

- [ ] **Step 3: 实现 Handler 与路由**

`AuthHandler.ChangeOpsPassword` 绑定 body、校验 8-72 位、按错误映射 400/403/500/503；注册到 `adminProtected.PUT("/auth/password", ...)`。测试 helper 增加 fake ops finder，供 `auth.Service` 使用。

- [ ] **Step 4: 运行 router 测试**

Run: `go test ./internal/router`
Expected: PASS。

---

### Task 3: ops-web API、校验纯函数与设置页

**Files:**
- Modify: `packages/ops-web/src/api/auth.ts`
- Modify: `packages/ops-web/src/utils/userAdmin.ts`
- Modify: `packages/ops-web/src/views/SettingsView.vue`
- Test: `packages/ops-web/src/utils/userAdmin.test.ts`
- Test: `packages/ops-web/src/api/auth.test.ts`

**Interfaces:**
- Produces:
  - `changePassword(oldPassword, newPassword): Promise<{ changed: boolean }>`
  - `validatePasswordChange(oldPassword, newPassword, confirm): string`

- [ ] **Step 1: 写 ops-web 失败测试**

`userAdmin.test.ts` 覆盖必填、8-72 位、两次不一致、新旧相同、通过；`auth.test.ts` 断言 `changePassword` 调用 `PUT /auth/password` 且 body 为 `{oldPassword,newPassword}`。

- [ ] **Step 2: 运行测试确认失败**

Run: `npm test -- --run src/utils/userAdmin.test.ts src/api/auth.test.ts`
Expected: FAIL，函数不存在。

- [ ] **Step 3: 实现 API 与校验纯函数**

在 `auth.ts` 增加 `changePassword`；在 `userAdmin.ts` 增加 `validatePasswordChange`。

- [ ] **Step 4: 实现设置页表单**

`SettingsView.vue` 顶部新增「账号安全」卡片；当前密码 / 新密码 / 确认新密码；提交中禁用；成功清空并提示；`ApiError.code === 40002` 显示「当前密码不正确」。页头说明改为账号安全 + 只读运行信息。

- [ ] **Step 5: 运行 ops-web 测试、类型检查、构建与 lint**

Run: `npm test -- --run && npm run build && npm run lint`
Expected: PASS。

---

### Task 4: 全量验证

**Files:** 无新增。

- [ ] **Step 1: 运行后端全量测试**

Run: `go test ./...`（工作目录 `packages/backend`）
Expected: PASS。

- [ ] **Step 2: 运行仓库质量门禁**

Run: `make validate`
Expected: `all checks passed` / `errors=0 warnings=0`。

- [ ] **Step 3: 记录结果**

在最终回复中给出测试命令与结果；不提交 git，除非用户明确要求。
