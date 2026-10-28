package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"vibebiz/internal/dao/model"
)

type fakeUserFinder struct {
	user           *model.User
	findErr        error
	workspaceIDs   []int64
	workspaceErr   error
	touchedID      int64
	touchedIP      string
	touchedAt      time.Time
	touchErr       error
	touchCallCount int
}

func (f *fakeUserFinder) FindUserByIdentifier(_ context.Context, _ string) (*model.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.user, nil
}

func (f *fakeUserFinder) WorkspaceIDs(_ context.Context, _ int64) ([]int64, error) {
	if f.workspaceErr != nil {
		return nil, f.workspaceErr
	}
	return f.workspaceIDs, nil
}

func (f *fakeUserFinder) TouchUserLogin(_ context.Context, id int64, ip string, at time.Time) error {
	f.touchedID = id
	f.touchedIP = ip
	f.touchedAt = at
	f.touchCallCount++
	return f.touchErr
}

type fakeOpsFinder struct {
	user           *model.OpsUser
	findErr        error
	touchedID      int64
	touchedIP      string
	touchedAt      time.Time
	touchErr       error
	touchCallCount int
}

func (f *fakeOpsFinder) FindOpsByIdentifier(_ context.Context, _ string) (*model.OpsUser, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.user, nil
}

func (f *fakeOpsFinder) TouchOpsLogin(_ context.Context, id int64, ip string, at time.Time) error {
	f.touchedID = id
	f.touchedIP = ip
	f.touchedAt = at
	f.touchCallCount++
	return f.touchErr
}

type fakeSessionStore struct {
	items   map[string]*Session
	err     error
	deleted []string
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{items: make(map[string]*Session)}
}

func sessionKey(realm Realm, id string) string { return string(realm) + ":" + id }

func (f *fakeSessionStore) Create(_ context.Context, session *Session, _ time.Duration) error {
	if f.err != nil {
		return f.err
	}
	copied := *session
	f.items[sessionKey(session.Realm, session.ID)] = &copied
	return nil
}

func (f *fakeSessionStore) Get(_ context.Context, realm Realm, id string) (*Session, error) {
	if f.err != nil {
		return nil, f.err
	}
	session, ok := f.items[sessionKey(realm, id)]
	if !ok {
		return nil, ErrSessionNotFound
	}
	copied := *session
	return &copied, nil
}

func (f *fakeSessionStore) Delete(_ context.Context, realm Realm, id string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, sessionKey(realm, id))
	delete(f.items, sessionKey(realm, id))
	return nil
}

func testService(t *testing.T, user *model.User, workspaceIDs []int64) (*Service, *fakeUserFinder, *fakeSessionStore) {
	t.Helper()
	hash, err := HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user.PasswordHash = &hash
	users := &fakeUserFinder{user: user, workspaceIDs: workspaceIDs}
	sessions := newFakeSessionStore()
	return &Service{
		Users:    users,
		Sessions: sessions,
		UserTTL:  168 * time.Hour,
		Now:      func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}, users, sessions
}

func TestLoginUserByUsernameAndEmail(t *testing.T) {
	for _, identifier := range []string{"wanghg11", "wang@example.com"} {
		user := &model.User{Base: model.Base{ID: 7}, Username: "wanghg11", Email: "wang@example.com", Status: 1}
		svc, users, sessions := testService(t, user, []int64{1, 2})

		session, err := svc.LoginUser(context.Background(), identifier, "secret-pass", "10.0.0.1")
		if err != nil {
			t.Fatalf("%s login: %v", identifier, err)
		}
		if session.Realm != RealmUser || session.UserID != 7 || session.Username != "wanghg11" || session.Email != "wang@example.com" {
			t.Fatalf("session = %+v", session)
		}
		if len(session.WorkspaceIDs) != 2 || session.WorkspaceIDs[0] != 1 {
			t.Fatalf("workspace ids = %v", session.WorkspaceIDs)
		}
		if session.ID == "" {
			t.Fatal("session id empty")
		}
		if _, ok := sessions.items[sessionKey(RealmUser, session.ID)]; !ok {
			t.Fatal("session not stored")
		}
		if users.touchCallCount != 1 || users.touchedIP != "10.0.0.1" || users.touchedID != 7 {
			t.Fatalf("touch = %+v", users)
		}
		if !users.touchedAt.Equal(session.CreatedAt) {
			t.Fatalf("touch at = %v, session at = %v", users.touchedAt, session.CreatedAt)
		}
	}
}

func TestLoginUserWrongPassword(t *testing.T) {
	svc, _, _ := testService(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, []int64{1})
	_, err := svc.LoginUser(context.Background(), "u", "wrong", "")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginUserUnknownAndNilHash(t *testing.T) {
	users := &fakeUserFinder{findErr: errors.New("not found")}
	svc := &Service{Users: users, Sessions: newFakeSessionStore(), Now: time.Now}
	if _, err := svc.LoginUser(context.Background(), "missing", "pw", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown err = %v, want ErrInvalidCredentials", err)
	}

	svc2, _, _ := testService(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Status: 1}, []int64{1})
	svc2.Users.(*fakeUserFinder).user.PasswordHash = nil
	if _, err := svc2.LoginUser(context.Background(), "u", "pw", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("nil hash err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginUserDisabled(t *testing.T) {
	svc, _, _ := testService(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 0}, []int64{1})
	if _, err := svc.LoginUser(context.Background(), "u", "secret-pass", ""); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
}

func TestLoginUserNoWorkspace(t *testing.T) {
	svc, _, _ := testService(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, nil)
	if _, err := svc.LoginUser(context.Background(), "u", "secret-pass", ""); !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("err = %v, want ErrNoWorkspace", err)
	}
}

func TestLoginOps(t *testing.T) {
	hash, err := HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ops := &fakeOpsFinder{user: &model.OpsUser{Base: model.Base{ID: 9}, Username: "admin", Email: "admin@example.com", RoleCode: "super_admin", Status: 1, PasswordHash: &hash}}
	sessions := newFakeSessionStore()
	svc := &Service{Ops: ops, Sessions: sessions, OpsTTL: 12 * time.Hour, Now: func() time.Time { return time.Unix(1700000000, 0).UTC() }}

	session, err := svc.LoginOps(context.Background(), "admin", "secret-pass", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if session.Realm != RealmOps || session.UserID != 9 || session.Role != "super_admin" {
		t.Fatalf("session = %+v", session)
	}
	if len(session.WorkspaceIDs) != 0 {
		t.Fatalf("ops session should not carry workspaces: %v", session.WorkspaceIDs)
	}
	if ops.touchCallCount != 1 || ops.touchedIP != "127.0.0.1" {
		t.Fatalf("ops touch = %+v", ops)
	}
}

func TestLoginOpsDisabled(t *testing.T) {
	hash, _ := HashPassword("secret-pass")
	ops := &fakeOpsFinder{user: &model.OpsUser{Base: model.Base{ID: 9}, Username: "admin", Status: 0, PasswordHash: &hash}}
	svc := &Service{Ops: ops, Sessions: newFakeSessionStore(), Now: time.Now}
	if _, err := svc.LoginOps(context.Background(), "admin", "secret-pass", ""); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
}

func TestCurrent(t *testing.T) {
	svc, _, sessions := testService(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, []int64{1})
	session, err := svc.LoginUser(context.Background(), "u", "secret-pass", "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Current(context.Background(), RealmUser, session.ID)
	if err != nil || got.ID != session.ID {
		t.Fatalf("current = %+v, err = %v", got, err)
	}
	if _, err := svc.Current(context.Background(), RealmOps, session.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("wrong realm err = %v", err)
	}
	if sessions.items == nil {
		t.Fatal("sessions missing")
	}
}

func TestLogout(t *testing.T) {
	svc, _, sessions := testService(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, []int64{1})
	session, err := svc.LoginUser(context.Background(), "u", "secret-pass", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(context.Background(), RealmUser, session.ID); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := svc.Current(context.Background(), RealmUser, session.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("after logout err = %v", err)
	}
	if err := svc.Logout(context.Background(), RealmUser, ""); err != nil {
		t.Fatalf("logout empty: %v", err)
	}
	if err := svc.Logout(context.Background(), RealmUser, session.ID); err != nil {
		t.Fatalf("logout twice: %v", err)
	}
	if len(sessions.deleted) < 2 {
		t.Fatalf("deleted = %v", sessions.deleted)
	}
}

func TestHashPasswordCost(t *testing.T) {
	hash, err := HashPassword("pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !VerifyPassword(hash, "pw") {
		t.Fatal("verify failed")
	}
	if VerifyPassword(hash, "other") {
		t.Fatal("wrong password verified")
	}
	if cost, err := bcrypt.Cost([]byte(hash)); err != nil || cost != 12 {
		t.Fatalf("cost = %d, err = %v", cost, err)
	}
}
