package model

type RagKnowledgeBase struct {
	Base
	UserID             int64  `gorm:"column:user_id" json:"userId"`
	Name               string `gorm:"column:name" json:"name"`
	RagDesc            string `gorm:"column:rag_desc" json:"ragDesc"`
	RagMetadata        JSON   `gorm:"column:rag_metadata;type:json" json:"ragMetadata,omitempty"`
	ChunkSize          int    `gorm:"column:chunk_size" json:"chunkSize"`
	OverlapSize        int    `gorm:"column:overlap_size" json:"overlapSize"`
	DimensionSize      int    `gorm:"column:dimension_size" json:"dimensionSize"`
	EmbeddingModelID   int64  `gorm:"column:embedding_model_id" json:"embeddingModelId"`
	EmbeddingModelName string `gorm:"column:embedding_model_name" json:"embeddingModelName"`
	RerankerModelID    *int64 `gorm:"column:reranker_model_id" json:"rerankerModelId,omitempty"`
	RerankerModelName  string `gorm:"column:reranker_model_name" json:"rerankerModelName"`
	DocCount           int64  `gorm:"column:doc_count" json:"docCount"`
	CollectionName     string `gorm:"column:collection_name" json:"collectionName"`
	Status             int8   `gorm:"column:status" json:"status"`
	CreatedBy          string `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy          string `gorm:"column:updated_by" json:"updatedBy"`
}

func (RagKnowledgeBase) TableName() string { return "rag_knowledge_base" }

type ProjectRag struct {
	Base
	WorkspaceID int64  `gorm:"column:workspace_id" json:"workspaceId"`
	ProjectID   int64  `gorm:"column:project_id" json:"projectId"`
	RagID       int64  `gorm:"column:rag_id" json:"ragId"`
	CreatedBy   string `gorm:"column:created_by" json:"createdBy"`
}

func (ProjectRag) TableName() string { return "project_rag" }
