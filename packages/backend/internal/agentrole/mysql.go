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

func (r *MySQLRepository) ListPage(ctx context.Context, query ListQuery) (Page, error) {
	query = normalizeListQuery(query)
	scope := func(db *gorm.DB) *gorm.DB {
		tx := db.Model(&daomodel.AgentRole{})
		if query.Keyword != "" {
			like := "%" + query.Keyword + "%"
			tx = tx.Where("(role_code LIKE ? OR role_name_en LIKE ? OR role_name_zh LIKE ?)", like, like, like)
		}
		return tx
	}
	var total int64
	if err := r.db.WithContext(ctx).Scopes(scope).Count(&total).Error; err != nil {
		return Page{}, err
	}
	var rows []daomodel.AgentRole
	if err := r.db.WithContext(ctx).Scopes(scope).
		Order("sort_order ASC, id ASC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&rows).Error; err != nil {
		return Page{}, err
	}
	items := make([]Role, 0, len(rows))
	for index := range rows {
		items = append(items, toRole(&rows[index]))
	}
	return Page{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
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

func (r *MySQLRepository) Update(ctx context.Context, id int64, input UpdateInput, updatedBy string) (Role, error) {
	result := r.db.WithContext(ctx).Model(&daomodel.AgentRole{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"description":   input.Description,
			"system_prompt": input.SystemPrompt,
			"status":        input.Status,
			"version":       gorm.Expr("version + 1"),
			"updated_by":    updatedBy,
		})
	if result.Error != nil {
		return Role{}, result.Error
	}
	return r.getByID(ctx, id)
}

func (r *MySQLRepository) getByID(ctx context.Context, id int64) (Role, error) {
	var row daomodel.AgentRole
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Role{}, ErrNotFound
		}
		return Role{}, err
	}
	return toRole(&row), nil
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
