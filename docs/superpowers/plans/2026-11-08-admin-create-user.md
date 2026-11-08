# Admin Create User Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an ops-web entry that creates a fully login-ready business user through a single transactional CP admin API.

**Architecture:** Extend `admin.Repository` with `CreateUser`; `MySQLRepository` writes `user`, personal `workspace`, `workspace_user`, and `user_quota` in one transaction, while `MemoryRepository` mirrors the behavior for local mode. A new `POST /cp/api/v1/admin/users` handler validates and normalizes input, hashes the password, and maps duplicate keys to 409. ops-web adds a dialog backed by a pure validator and a typed API call.

**Tech Stack:** Go 1.24 + Echo v4 + GORM + MySQL (go-sql-driver), sqlmock, Vue 3 + TypeScript + PrimeVue + Vitest.

**Spec:** `docs/superpowers/specs/2026-11-08-admin-create-user-design.md`

## Global Constraints

- Colors are limited to black / white / gray / blue.
- Do not add database migrations; the four target tables already exist.
- Username: trimmed, 3–32 chars, `[A-Za-z0-9_-]`, unique.
- Password: 8–72 bytes; bcrypt via `auth.HashPassword`.
- Email: trimmed + lowercased; empty becomes `<username>@test.local`; max 255; unique.
- Nickname: trimmed; empty becomes username; max 255 runes.
- Plan: `free` / `pro` / `max`, default `free`.
- Status: `0` / `1`, default `1`; receive as `*int8` to distinguish omitted.
- Success envelope `{code:0,message,result}`; conflict `40901`; invalid input `40001`.
- Do not commit unless the user explicitly asks. Use a test/verification checkpoint instead of a git commit after each task.

---

### Task 1: Repository contract + MemoryRepository

**Files:**
- Modify: `packages/backend/internal/admin/repository.go`
- Modify: `packages/backend/internal/admin/memory.go`
- Test: `packages/backend/internal/admin/memory_test.go` (create)

**Interfaces:**
- Produces:
  - `admin.ErrConflict`
  - `admin.CreateUserInput{Username, Email, Nickname, PasswordHash, Plan string; Status int8; Operator string; At time.Time}`
  - `admin.Repository.CreateUser(ctx context.Context, input CreateUserInput) (UserDetail, error)`

- [ ] **Step 1: Write the failing memory tests**

Create `packages/backend/internal/admin/memory_test.go`:

```go
package admin

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryCreateUser(t *testing.T) {
	repo := NewMemoryRepository().(*MemoryRepository)
	at := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	detail, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username:     "tester01",
		Email:        "tester01@example.com",
		Nickname:     "Tester",
		PasswordHash: "hash",
		Plan:         "pro",
		Status:       1,
		Operator:     "admin",
		At:           at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "3" || detail.Username != "tester01" || detail.Name != "Tester" ||
		detail.Email != "tester01@example.com" || detail.Plan != "pro" ||
		detail.Source != "ADMIN" || detail.Status != "active" ||
		detail.EmailVerifiedAt == nil {
		t.Fatalf("detail = %+v", detail)
	}
	if repo.details[detail.ID].Username != "tester01" || repo.passwordHash[detail.ID] != "hash" {
		t.Fatalf("memory state = %+v %+v", repo.details, repo.passwordHash)
	}
}

func TestMemoryCreateUserRejectsDuplicate(t *testing.T) {
	repo := NewMemoryRepository().(*MemoryRepository)
	_, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "wanghg11", Email: "new@example.com", Nickname: "dup", Plan: "free", Status: 1,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/admin -run TestMemoryCreateUser -v`
Expected: FAIL — `CreateUser` is not defined / `MemoryRepository` does not implement `Repository`.

- [ ] **Step 3: Implement the contract and memory behavior**

In `packages/backend/internal/admin/repository.go`, replace the error declaration with:

```go
var (
	ErrNotFound = errors.New("admin record not found")
	ErrConflict = errors.New("admin record already exists")
)
```

Add after `UserStatusUpdate`:

```go
type CreateUserInput struct {
	Username     string
	Email        string
	Nickname     string
	PasswordHash string
	Plan         string
	Status       int8
	Operator     string
	At           time.Time
}
```

Add to `Repository`:

```go
CreateUser(ctx context.Context, input CreateUserInput) (UserDetail, error)
```

In `packages/backend/internal/admin/memory.go`, add `"strings"` to imports and implement:

```go
func (r *MemoryRepository) CreateUser(_ context.Context, input CreateUserInput) (UserDetail, error) {
	for _, detail := range r.details {
		if strings.EqualFold(detail.Username, input.Username) || strings.EqualFold(detail.Email, input.Email) {
			return UserDetail{}, ErrConflict
		}
	}
	var maxID int64
	for _, item := range r.users {
		if id, err := strconv.ParseInt(item.ID, 10, 64); err == nil && id > maxID {
			maxID = id
		}
	}
	id := strconv.FormatInt(maxID+1, 10)
	status := "disabled"
	if input.Status == 1 {
		status = "active"
	}
	name := input.Nickname
	if name == "" {
		name = input.Username
	}
	now := input.At
	if now.IsZero() {
		now = time.Now().UTC()
	}
	detail := UserDetail{
		ID: id, Username: input.Username, Name: name, Nickname: input.Nickname,
		Email: input.Email, Plan: input.Plan, Source: "ADMIN", Status: status,
		CreatedAt: now, UpdatedAt: now,
	}
	if input.Email != "" {
		detail.EmailVerifiedAt = &now
	}
	r.users = append(r.users, User{
		ID: id, Name: name, Email: input.Email, Plan: input.Plan, Status: status, CreatedAt: now,
	})
	r.details[id] = detail
	r.passwordHash[id] = input.PasswordHash
	return detail, nil
}
```

Also change `MemoryRepository.Overview` from `Users: 2` to `Users: len(r.users)`.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/admin -run TestMemoryCreateUser -v`
Expected: PASS.

- [ ] **Step 5: Checkpoint**

Run: `go test ./internal/admin`
Expected: PASS. Do not commit.

---

### Task 2: MySQLRepository transaction

**Files:**
- Modify: `packages/backend/internal/admin/mysql.go`
- Test: `packages/backend/internal/admin/mysql_test.go`

**Interfaces:**
- Consumes: `admin.CreateUserInput`, `admin.ErrConflict`.
- Produces: `(*MySQLRepository).CreateUser`.

- [ ] **Step 1: Write the failing sqlmock tests**

Append to `packages/backend/internal/admin/mysql_test.go` (add `mysqldriver "github.com/go-sql-driver/mysql"` to imports):

```go
func TestMySQLRepositoryCreateUser(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	created := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO `workspace`").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO `workspace_user`").WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("INSERT INTO `user_quota`").WillReturnResult(sqlmock.NewResult(13, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT u.id, u.username, u.nickname").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "nickname", "email", "phone_number", "avatar_url", "avatar_s3_path",
			"plan_code", "source", "status", "email_verified_at", "last_login_time", "last_login_ip",
			"disabled_reason", "disabled_at", "disabled_by", "created_at", "updated_at",
		}).AddRow(
			7, "tester01", "Tester", "tester01@example.com", "", "", "",
			"pro", "ADMIN", 1, created, nil, "",
			"", nil, "", created, created,
		))

	detail, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "tester01", Email: "tester01@example.com", Nickname: "Tester",
		PasswordHash: "hash", Plan: "pro", Status: 1, Operator: "admin", At: created,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "7" || detail.Username != "tester01" || detail.Plan != "pro" || detail.Status != "active" {
		t.Fatalf("detail = %+v", detail)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCreateUserConflict(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user`").
		WillReturnError(&mysqldriver.MySQLError{Number: 1062})
	mock.ExpectRollback()

	_, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "tester01", Email: "tester01@example.com", Plan: "free", Status: 1,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/admin -run TestMySQLRepositoryCreateUser -v`
Expected: FAIL — `CreateUser` undefined.

- [ ] **Step 3: Implement the transaction**

In `packages/backend/internal/admin/mysql.go`, add imports `"errors"` and `mysqldriver "github.com/go-sql-driver/mysql"`, then add:

```go
func (r *MySQLRepository) CreateUser(ctx context.Context, input CreateUserInput) (UserDetail, error) {
	now := input.At
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var userID int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user := model.User{
			Email: input.Email, Source: "ADMIN", Username: input.Username,
			PasswordHash: &input.PasswordHash, Nickname: input.Nickname, Status: input.Status,
		}
		if input.Email != "" {
			user.EmailVerifiedAt = &now
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		workspace := model.Workspace{
			OwnerUserID: user.ID, Name: input.Username + " 的个人工作区",
			Description: "默认个人工作区", WorkspaceType: "personal",
			Visibility: "private", Status: 1, CreatedBy: input.Operator,
		}
		if err := tx.Create(&workspace).Error; err != nil {
			return err
		}
		member := model.WorkspaceUser{
			WorkspaceID: workspace.ID, UserID: user.ID, Role: "owner",
			Status: 1, JoinedAt: &now,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		quota := model.UserQuota{UserID: user.ID, PlanCode: input.Plan, Status: 1}
		if err := tx.Create(&quota).Error; err != nil {
			return err
		}
		userID = user.ID
		return nil
	})
	if err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return UserDetail{}, ErrConflict
		}
		return UserDetail{}, fmt.Errorf("create admin user: %w", err)
	}
	return r.User(ctx, userID)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/admin -run TestMySQLRepositoryCreateUser -v`
Expected: PASS.

- [ ] **Step 5: Checkpoint**

Run: `go test ./internal/admin`
Expected: PASS. Do not commit.

---

### Task 3: Handler validation, route, error mapping

**Files:**
- Modify: `packages/backend/internal/handler/admin.go`
- Modify: `packages/backend/internal/router/control.go`
- Test: `packages/backend/internal/handler/admin_user_test.go`

**Interfaces:**
- Consumes: `admin.Repository.CreateUser`, `admin.ErrConflict`.
- Produces: `(*AdminHandler).CreateUser`, route `POST /cp/api/v1/admin/users`.

- [ ] **Step 1: Extend the fake and write failing handler tests**

In `packages/backend/internal/handler/admin_user_test.go`, add fields to `fakeAdminRepository`:

```go
	createInput  admin.CreateUserInput
	createDetail admin.UserDetail
	createErr    error
```

Add the method:

```go
func (f *fakeAdminRepository) CreateUser(_ context.Context, input admin.CreateUserInput) (admin.UserDetail, error) {
	f.createInput = input
	if f.createErr != nil {
		return admin.UserDetail{}, f.createErr
	}
	return f.createDetail, nil
}
```

Append tests:

```go
func TestAdminCreateUserNormalizesInput(t *testing.T) {
	repo := &fakeAdminRepository{createDetail: admin.UserDetail{ID: "7", Username: "tester01", Status: "active"}}
	h := newAdminUserHandler(repo)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/users", h.CreateUser, &auth.Session{
		ID: "sid", Realm: auth.RealmOps, UserID: 3, Username: "admin",
	})

	rec := adminUserRequest(e, http.MethodPost, "/users",
		`{"username":" tester01 ","password":"secret123","email":" TESTER01@Example.com ","nickname":"","plan":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	in := repo.createInput
	if in.Username != "tester01" || in.Email != "tester01@example.com" || in.Nickname != "tester01" ||
		in.Plan != "free" || in.Status != 1 || in.Operator != "admin" {
		t.Fatalf("input = %+v", in)
	}
	if !auth.VerifyPassword(in.PasswordHash, "secret123") {
		t.Fatalf("password hash = %q", in.PasswordHash)
	}
}

func TestAdminCreateUserGeneratesEmail(t *testing.T) {
	repo := &fakeAdminRepository{createDetail: admin.UserDetail{ID: "7"}}
	h := newAdminUserHandler(repo)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/users", h.CreateUser, &auth.Session{
		ID: "sid", Realm: auth.RealmOps, Username: "admin",
	})

	rec := adminUserRequest(e, http.MethodPost, "/users", `{"username":"tester01","password":"secret123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.createInput.Email != "tester01@test.local" {
		t.Fatalf("email = %q", repo.createInput.Email)
	}
}

func TestAdminCreateUserRejectsInvalidInput(t *testing.T) {
	cases := []string{
		`{"username":"ab","password":"secret123"}`,
		`{"username":"bad name","password":"secret123"}`,
		`{"username":"tester01","password":"short"}`,
		`{"username":"tester01","password":"secret123","email":"not-an-email"}`,
		`{"username":"tester01","password":"secret123","plan":"vip"}`,
		`{"username":"tester01","password":"secret123","status":2}`,
	}
	for _, body := range cases {
		repo := &fakeAdminRepository{}
		h := newAdminUserHandler(repo)
		e := echo.New()
		routeWithSession(e, http.MethodPost, "/users", h.CreateUser, &auth.Session{Realm: auth.RealmOps})
		rec := adminUserRequest(e, http.MethodPost, "/users", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, response = %s", body, rec.Code, rec.Body.String())
		}
		if repo.createInput.Username != "" {
			t.Fatalf("body %s: repository should not be called", body)
		}
	}
}

func TestAdminCreateUserConflict(t *testing.T) {
	repo := &fakeAdminRepository{createErr: admin.ErrConflict}
	h := newAdminUserHandler(repo)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/users", h.CreateUser, &auth.Session{Realm: auth.RealmOps})

	rec := adminUserRequest(e, http.MethodPost, "/users", `{"username":"tester01","password":"secret123"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "40901") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/handler -run TestAdminCreateUser -v`
Expected: FAIL — `CreateUser` undefined.

- [ ] **Step 3: Implement handler and route**

In `packages/backend/internal/handler/admin.go`, add imports `"net/mail"` and `"regexp"`. Add near the other request types:

```go
var adminUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Plan     string `json:"plan"`
	Status   *int8  `json:"status"`
}

func (h *AdminHandler) CreateUser(c echo.Context) error {
	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	username := strings.TrimSpace(req.Username)
	if !adminUsernamePattern.MatchString(username) {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		email = strings.ToLower(username) + "@test.local"
	}
	if len(email) > 255 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	if addr, err := mail.ParseAddress(email); err != nil || !strings.EqualFold(addr.Address, email) {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = username
	}
	if utf8.RuneCountInString(nickname) > 255 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	plan := strings.TrimSpace(req.Plan)
	if plan == "" {
		plan = "free"
	}
	switch plan {
	case "free", "pro", "max":
	default:
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	if status != 0 && status != 1 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user input")
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "hash password failed")
	}
	operator := ""
	if session, ok := auth.SessionFrom(c); ok {
		operator = session.Username
	}
	detail, err := h.Repo.CreateUser(c.Request().Context(), admin.CreateUserInput{
		Username: username, Email: email, Nickname: nickname, PasswordHash: hash,
		Plan: plan, Status: status, Operator: operator, At: h.now(),
	})
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, detail)
}
```

In `adminRepoError`, add before the fallback:

```go
	if errors.Is(err, admin.ErrConflict) {
		return response.Fail(c, http.StatusConflict, 40901, "username or email already exists")
	}
```

In `packages/backend/internal/router/control.go`, register:

```go
	adminProtected.POST("/users", adminHandler.CreateUser)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/handler -run TestAdminCreateUser -v`
Expected: PASS.

- [ ] **Step 5: Checkpoint**

Run: `go test ./internal/handler ./internal/admin ./internal/router`
Expected: PASS. Do not commit.

---

### Task 4: ops-web validator, types, and API

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Modify: `packages/ops-web/src/utils/userAdmin.ts`
- Test: `packages/ops-web/src/utils/userAdmin.test.ts`
- Test: `packages/ops-web/src/api/adminWrites.test.ts`

**Interfaces:**
- Produces:
  - `CreateUserInput{username,password,email?,nickname?,plan,status}`
  - `validateCreateUser(form: CreateUserForm): string`
  - `createUser(input: CreateUserInput): Promise<AdminUserDetail>`

- [ ] **Step 1: Write the failing tests**

Append to `packages/ops-web/src/utils/userAdmin.test.ts` (add `validateCreateUser` to the existing import):

```ts
describe('create user validation', () => {
  const valid = {
    username: 'tester01',
    password: 'secret123',
    confirmPassword: 'secret123',
    email: '',
    nickname: '',
    plan: 'free',
    status: 1,
  }

  it('accepts a valid form', () => {
    expect(validateCreateUser(valid)).toBe('')
  })

  it('validates the username', () => {
    expect(validateCreateUser({ ...valid, username: 'ab' })).toBe(
      '用户名需为 3-32 位字母、数字、下划线或中划线',
    )
    expect(validateCreateUser({ ...valid, username: 'bad name' })).toBe(
      '用户名需为 3-32 位字母、数字、下划线或中划线',
    )
  })

  it('validates the password', () => {
    expect(validateCreateUser({ ...valid, password: 'short', confirmPassword: 'short' })).toBe(
      '密码至少 8 位',
    )
    expect(
      validateCreateUser({
        ...valid,
        password: 'x'.repeat(73),
        confirmPassword: 'x'.repeat(73),
      }),
    ).toBe('密码不能超过 72 位')
    expect(validateCreateUser({ ...valid, confirmPassword: 'other' })).toBe('两次密码不一致')
  })

  it('validates the optional email', () => {
    expect(validateCreateUser({ ...valid, email: 'not-an-email' })).toBe('请输入有效的邮箱')
    expect(validateCreateUser({ ...valid, email: 'tester01@example.com' })).toBe('')
  })

  it('validates the plan and status', () => {
    expect(validateCreateUser({ ...valid, plan: 'vip' })).toBe('请选择套餐')
    expect(validateCreateUser({ ...valid, status: 2 })).toBe('请选择状态')
  })
})
```

Append to `packages/ops-web/src/api/adminWrites.test.ts` (add `createUser` to the import):

```ts
  it('creates a user', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: '7', username: 'tester01' } },
    })
    await createUser({
      username: 'tester01',
      password: 'secret123',
      email: 'tester01@example.com',
      nickname: 'Tester',
      plan: 'free',
      status: 1,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'post',
      url: '/users',
      data: {
        username: 'tester01',
        password: 'secret123',
        email: 'tester01@example.com',
        nickname: 'Tester',
        plan: 'free',
        status: 1,
      },
    })
  })
```

- [ ] **Step 2: Run tests to verify failure**

Run: `npm test -- src/utils/userAdmin.test.ts src/api/adminWrites.test.ts`
Expected: FAIL — `validateCreateUser` / `createUser` not exported.

- [ ] **Step 3: Implement types, validator, and API**

In `packages/ops-web/src/types/index.ts`, after `AdminUserDetail`, add:

```ts
export interface CreateUserInput {
  username: string
  password: string
  email?: string
  nickname?: string
  plan: string
  status: number
}
```

In `packages/ops-web/src/utils/userAdmin.ts`, append:

```ts
export interface CreateUserForm {
  username: string
  password: string
  confirmPassword: string
  email: string
  nickname: string
  plan: string
  status: number
}

const usernamePattern = /^[A-Za-z0-9_-]{3,32}$/
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function validateCreateUser(form: CreateUserForm): string {
  if (!usernamePattern.test(form.username.trim())) {
    return '用户名需为 3-32 位字母、数字、下划线或中划线'
  }
  if (form.password.length < 8) {
    return '密码至少 8 位'
  }
  if (form.password.length > 72) {
    return '密码不能超过 72 位'
  }
  if (form.password !== form.confirmPassword) {
    return '两次密码不一致'
  }
  const email = form.email.trim()
  if (email && (email.length > 255 || !emailPattern.test(email))) {
    return '请输入有效的邮箱'
  }
  if ([...form.nickname.trim()].length > 255) {
    return '昵称不能超过 255 字'
  }
  if (!['free', 'pro', 'max'].includes(form.plan)) {
    return '请选择套餐'
  }
  if (form.status !== 0 && form.status !== 1) {
    return '请选择状态'
  }
  return ''
}
```

In `packages/ops-web/src/api/admin.ts`, add `CreateUserInput` to the type import and append:

```ts
export async function createUser(input: CreateUserInput): Promise<AdminUserDetail> {
  return request<AdminUserDetail>({ method: 'post', url: '/users', data: input })
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `npm test -- src/utils/userAdmin.test.ts src/api/adminWrites.test.ts`
Expected: PASS.

- [ ] **Step 5: Checkpoint**

Run: `npm test`
Expected: PASS. Do not commit.

---

### Task 5: UsersView dialog

**Files:**
- Modify: `packages/ops-web/src/views/UsersView.vue`

**Interfaces:**
- Consumes: `createUser`, `CreateUserInput`, `validateCreateUser`.

- [ ] **Step 1: Add script state and submit logic**

Add imports:

```ts
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { createUser, getUser, getUsers, updateUserPassword, updateUserStatus } from '@/api/admin'
import type { AdminUser, AdminUserDetail, CreateUserInput, TableColumn } from '@/types'
import { validateCreateUser, validateDisableReason, validateNewPassword } from '@/utils/userAdmin'
```

Add after existing dialog state:

```ts
const planOptions = [
  { label: '免费 free', value: 'free' },
  { label: '专业 pro', value: 'pro' },
  { label: '旗舰 max', value: 'max' },
]

const createVisible = ref(false)
const creating = ref(false)
const createError = ref('')
const createForm = ref({
  username: '',
  password: '',
  confirmPassword: '',
  email: '',
  nickname: '',
  plan: 'free',
  status: 1,
})

function openCreate() {
  createForm.value = {
    username: '',
    password: '',
    confirmPassword: '',
    email: '',
    nickname: '',
    plan: 'free',
    status: 1,
  }
  createError.value = ''
  createVisible.value = true
}

async function submitCreate() {
  const form = createForm.value
  const message = validateCreateUser(form)
  if (message) {
    createError.value = message
    return
  }
  creating.value = true
  createError.value = ''
  try {
    const input: CreateUserInput = {
      username: form.username.trim(),
      password: form.password,
      email: form.email.trim() || undefined,
      nickname: form.nickname.trim() || undefined,
      plan: form.plan,
      status: form.status,
    }
    await createUser(input)
    notifySuccess('用户已创建')
    createVisible.value = false
    page.value = 1
    await load()
  } catch (err) {
    createError.value = err instanceof Error ? err.message : '创建用户失败'
  } finally {
    creating.value = false
  }
}
```

- [ ] **Step 2: Add the header button and dialog template**

Replace the page header's trailing `<span class="tag">{{ total }} 条</span>` with:

```html
      <div class="page-actions">
        <span class="tag">{{ total }} 条</span>
        <Button label="添加用户" icon="pi pi-plus" @click="openCreate" />
      </div>
```

Add this dialog before the disable dialog:

```html
    <Dialog
      v-model:visible="createVisible"
      header="添加用户"
      modal
      :style="{ width: '520px' }"
      @hide="createError = ''"
    >
      <div class="form">
        <label class="field">
          <span>用户名</span>
          <InputText v-model="createForm.username" maxlength="32" placeholder="3-32 位字母、数字、_ 或 -" />
        </label>
        <label class="field">
          <span>密码</span>
          <Password v-model="createForm.password" toggle-mask :feedback="false" autocomplete="new-password" />
        </label>
        <label class="field">
          <span>确认密码</span>
          <Password
            v-model="createForm.confirmPassword"
            toggle-mask
            :feedback="false"
            autocomplete="new-password"
          />
        </label>
        <label class="field">
          <span>邮箱</span>
          <InputText v-model="createForm.email" maxlength="255" placeholder="留空自动生成" />
        </label>
        <label class="field">
          <span>昵称</span>
          <InputText v-model="createForm.nickname" maxlength="255" placeholder="留空使用用户名" />
        </label>
        <label class="field">
          <span>套餐</span>
          <Select
            v-model="createForm.plan"
            :options="planOptions"
            option-label="label"
            option-value="value"
          />
        </label>
        <label class="field">
          <span>状态</span>
          <Select
            v-model="createForm.status"
            :options="[
              { label: '启用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
            option-label="label"
            option-value="value"
          />
        </label>
        <p class="hint">邮箱留空将自动生成 &lt;用户名&gt;@test.local。</p>
        <p v-if="createError" class="error-text">{{ createError }}</p>
      </div>
      <template #footer>
        <Button label="取消" severity="secondary" text @click="createVisible = false" />
        <Button label="创建" :loading="creating" @click="submitCreate" />
      </template>
    </Dialog>
```

Add scoped style:

```css
.page-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
```

- [ ] **Step 3: Run typecheck and lint**

Run: `npm run build`
Expected: PASS.

Run: `npm run lint`
Expected: PASS.

- [ ] **Step 4: Run tests**

Run: `npm test`
Expected: PASS.

- [ ] **Step 5: Checkpoint**

Do not commit.

---

### Task 6: Full verification

- [ ] **Step 1: Backend suite**

Run: `go test ./...`
Workdir: `packages/backend`
Expected: PASS.

- [ ] **Step 2: ops-web suite**

Run: `npm test && npm run build && npm run lint`
Workdir: `packages/ops-web`
Expected: PASS.

- [ ] **Step 3: Repository gate**

Run: `make validate`
Workdir: repository root
Expected: PASS.

- [ ] **Step 4: Manual smoke (optional)**

1. Start `make dev`.
2. Login to ops-web as the local admin.
3. Open 用户 → 添加用户, create `tester01`.
4. Confirm the row appears; login to web as `tester01` with the chosen password.
5. Repeat the same username and confirm the dialog shows the 409 message.

Do not commit unless the user explicitly asks.
