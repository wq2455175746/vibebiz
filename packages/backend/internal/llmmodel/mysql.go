package llmmodel

import (
	"context"
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
)

const (
	scopePlatform = "platform"
	platformUser  = 0
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) List(ctx context.Context) ([]Model, error) {
	var rows []daomodel.LLMModel
	err := r.db.WithContext(ctx).
		Where("scope = ? AND user_id = ?", scopePlatform, platformUser).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(rows))
	for index := range rows {
		out = append(out, toModel(&rows[index]))
	}
	return out, nil
}

func (r *MySQLRepository) Create(ctx context.Context, input CreateInput, apiKeyCipher, createdBy string) (Model, error) {
	var result Model
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.IsDefault {
			if err := tx.Model(&daomodel.LLMModel{}).
				Where("scope = ? AND user_id = ? AND model_type = ? AND is_default = ?", scopePlatform, platformUser, input.ModelType, 1).
				Update("is_default", 0).Error; err != nil {
				return err
			}
		}
		row := &daomodel.LLMModel{
			Scope: scopePlatform, UserID: platformUser,
			ModelName: input.ModelName, ModelCode: input.ModelCode, APIURL: input.APIURL,
			APIKeyCipher: apiKeyCipher, ProviderCode: input.ProviderCode, ModelType: input.ModelType,
			MaxTokens: input.MaxTokens, IsDefault: boolToInt8(input.IsDefault), Status: input.Status,
			ExtraParams: input.ExtraParams, CreatedBy: createdBy,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		result = toModel(row)
		return nil
	})
	if err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return Model{}, ErrConflict
		}
		return Model{}, err
	}
	return result, nil
}

func toModel(row *daomodel.LLMModel) Model {
	return Model{
		ID: row.ID, ModelName: row.ModelName, ModelCode: row.ModelCode, APIURL: row.APIURL,
		ProviderCode: row.ProviderCode, ModelType: row.ModelType, MaxTokens: row.MaxTokens,
		IsDefault: row.IsDefault == 1, Status: row.Status, HasAPIKey: row.APIKeyCipher != "",
		ExtraParams: row.ExtraParams, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func boolToInt8(value bool) int8 {
	if value {
		return 1
	}
	return 0
}
