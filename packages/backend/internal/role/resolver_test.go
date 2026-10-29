package role

import (
	"context"
	"errors"
	"testing"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/model"
)

type fakeProjects struct{ project *model.Project }

func (f fakeProjects) Get(context.Context, string) (*model.Project, error) {
	return f.project, nil
}

type fakeRoles struct{ roles []agentrole.Role }

func (f fakeRoles) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, nil
}

func (f fakeRoles) GetActive(_ context.Context, id int64) (agentrole.Role, error) {
	for _, role := range f.roles {
		if role.ID == id {
			return role, nil
		}
	}
	return agentrole.Role{}, agentrole.ErrNotFound
}

func TestResolverExpandsTeamToAllNormalRoles(t *testing.T) {
	resolver := NewResolver(
		fakeProjects{project: &model.Project{ID: "p1", AgentRoleIDs: []int64{6}}},
		fakeRoles{roles: []agentrole.Role{
			{ID: 6, RoleCode: "team", Status: 1},
			{ID: 1, RoleCode: "alex", Status: 1},
			{ID: 4, RoleCode: "emma", Status: 1},
		}},
	)
	got, err := resolver.Resolve(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.TeamMode || len(got.Roles) != 3 {
		t.Fatalf("context = %+v", got)
	}
	if got.Roles[0].ID != 6 {
		t.Fatalf("team must be first: %+v", got.Roles)
	}
}

func TestResolverFallsBackToDefaultRole(t *testing.T) {
	first := int64(1)
	resolver := NewResolver(
		fakeProjects{project: &model.Project{ID: "p1", DefaultAgentRoleID: &first}},
		fakeRoles{roles: []agentrole.Role{{ID: 1, RoleCode: "alex", Status: 1}}},
	)
	got, err := resolver.Resolve(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roles) != 1 || got.Roles[0].ID != 1 || got.TeamMode {
		t.Fatalf("context = %+v", got)
	}
}

type failingRoles struct{ err error }

func (f failingRoles) ListActive(context.Context) ([]agentrole.Role, error) {
	return nil, f.err
}

func (f failingRoles) GetActive(context.Context, int64) (agentrole.Role, error) {
	return agentrole.Role{}, f.err
}

func TestResolverReturnsNonNotFoundRoleError(t *testing.T) {
	boom := errors.New("role lookup failed")
	resolver := NewResolver(
		fakeProjects{project: &model.Project{ID: "p1", AgentRoleIDs: []int64{1}}},
		failingRoles{err: boom},
	)
	got, err := resolver.Resolve(context.Background(), "p1")
	if !errors.Is(err, boom) || got != nil {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}
