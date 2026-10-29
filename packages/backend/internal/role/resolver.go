package role

import (
	"context"
	"errors"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/model"
)

type ProjectStore interface {
	Get(ctx context.Context, id string) (*model.Project, error)
}

type RoleStore interface {
	ListActive(ctx context.Context) ([]agentrole.Role, error)
	GetActive(ctx context.Context, id int64) (agentrole.Role, error)
}

type Context struct {
	Roles    []agentrole.Role
	TeamMode bool
}

type Resolver struct {
	Projects ProjectStore
	Roles    RoleStore
}

func NewResolver(projects ProjectStore, roles RoleStore) *Resolver {
	return &Resolver{Projects: projects, Roles: roles}
}

func (r *Resolver) Resolve(ctx context.Context, projectID string) (*Context, error) {
	project, err := r.Projects.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := append([]int64(nil), project.AgentRoleIDs...)
	if len(ids) == 0 && project.DefaultAgentRoleID != nil {
		ids = []int64{*project.DefaultAgentRoleID}
	}
	if len(ids) == 0 {
		return &Context{}, nil
	}
	roles := make([]agentrole.Role, 0, len(ids))
	hasTeam := false
	for _, id := range ids {
		role, err := r.Roles.GetActive(ctx, id)
		if err != nil {
			if errors.Is(err, agentrole.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if role.RoleCode == "team" {
			hasTeam = true
			roles = append(roles, role)
			continue
		}
		roles = append(roles, role)
	}
	if hasTeam {
		all, err := r.Roles.ListActive(ctx)
		if err != nil {
			return nil, err
		}
		expanded := make([]agentrole.Role, 0, len(all))
		var team *agentrole.Role
		for i := range all {
			if all[i].RoleCode == "team" {
				copied := all[i]
				team = &copied
				continue
			}
			expanded = append(expanded, all[i])
		}
		if team != nil {
			expanded = append([]agentrole.Role{*team}, expanded...)
		}
		roles = expanded
	}
	return &Context{Roles: roles, TeamMode: hasTeam}, nil
}
