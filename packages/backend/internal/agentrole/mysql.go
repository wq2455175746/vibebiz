package agentrole

import (
	"context"
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) List(ctx context.Context) ([]Role, error) {
	var rows []daomodel.AgentRole
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Role, 0, len(rows))
	for index := range rows {
		out = append(out, toRole(&rows[index]))
	}
	return out, nil
}

func (r *MySQLRepository) Create(ctx context.Context, input CreateInput, createdBy string) (Role, error) {
	row := &daomodel.AgentRole{
		RoleCode: input.RoleCode, RoleNameEN: input.RoleNameEN, RoleNameZH: input.RoleNameZH,
		Description: input.Description, SystemPrompt: input.SystemPrompt, AvatarURL: input.AvatarKey,
		IsBuiltin: 0, Status: input.Status, SortOrder: input.SortOrder, Version: 1,
		CreatedBy: createdBy,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return Role{}, ErrConflict
		}
		return Role{}, err
	}
	return toRole(row), nil
}

func toRole(row *daomodel.AgentRole) Role {
	return Role{
		ID: row.ID, RoleCode: row.RoleCode, RoleNameEN: row.RoleNameEN, RoleNameZH: row.RoleNameZH,
		Description: row.Description, SystemPrompt: row.SystemPrompt, AvatarURL: row.AvatarURL,
		IsBuiltin: row.IsBuiltin == 1, Status: row.Status, SortOrder: row.SortOrder,
		Version: row.Version, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (r *MySQLRepository) ListActive(ctx context.Context) ([]Role, error) {
	var rows []daomodel.AgentRole
	if err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Role, 0, len(rows))
	for index := range rows {
		out = append(out, toRole(&rows[index]))
	}
	return out, nil
}

func (r *MySQLRepository) GetActive(ctx context.Context, id int64) (Role, error) {
	var row daomodel.AgentRole
	if err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Where("id = ?", id).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Role{}, ErrNotFound
		}
		return Role{}, err
	}
	return toRole(&row), nil
}
