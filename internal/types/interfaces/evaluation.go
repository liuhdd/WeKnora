package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// EvaluationService defines operations for evaluation tasks
type EvaluationService interface {
	// Evaluation starts a new evaluation task
	Evaluation(ctx context.Context, datasetID string, datasetVersion string, knowledgeBaseID string,
		chatModelID string, rerankModelID string,
	) (*types.EvaluationDetail, error)
	// EvaluationResult retrieves evaluation result by task ID
	EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error)
	ListEvaluationRuns(ctx context.Context, page int, pageSize int) (*types.EvaluationRunPage, error)
	ListEvaluationDatasets(ctx context.Context) ([]types.EvaluationDatasetSummary, error)
	GetEvaluationDataset(ctx context.Context, datasetID string) (*types.EvaluationDataset, error)
	GetEvaluationDatasetVersion(ctx context.Context, datasetID string, version string) (*types.EvaluationDataset, error)
}

// EvaluationDatasetRepository 管理租户隔离的草稿和不可变发布版 / Stores tenant-scoped drafts and immutable published versions.
type EvaluationDatasetRepository interface {
	Create(ctx context.Context, tenantID uint64, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error)
	GetDraft(ctx context.Context, tenantID uint64, datasetID string) (*types.EvaluationDatasetDraft, error)
	SaveDraft(ctx context.Context, tenantID uint64, datasetID string, revision int, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error)
	Publish(ctx context.Context, tenantID uint64, datasetID string, revision int) (*types.EvaluationDatasetVersion, error)
	GetPublished(ctx context.Context, tenantID uint64, datasetID string, version int) (*types.EvaluationPublishedDataset, error)
	ListVersions(ctx context.Context, tenantID uint64, datasetID string) ([]types.EvaluationDatasetVersion, error)
	List(ctx context.Context, tenantID uint64) ([]types.EvaluationDatasetSummary, error)
}

// EvaluationRepository 保存隔离的评测历史 / Persists tenant-isolated evaluation history.
type EvaluationRepository interface {
	Save(ctx context.Context, detail *types.EvaluationDetail) error
	Get(ctx context.Context, tenantID uint64, taskID string) (*types.EvaluationDetail, error)
	List(ctx context.Context, tenantID uint64, page int, pageSize int) (*types.EvaluationRunPage, error)
	Heartbeat(ctx context.Context, tenantID uint64, taskID string) error
}

// Metrics defines interface for computing evaluation metrics
type Metrics interface {
	// Compute calculates metric score based on input data
	Compute(metricInput *types.MetricInput) float64
}

// EvalHook defines interface for evaluation process hooks
type EvalHook interface {
	// Handle processes evaluation state change
	Handle(ctx context.Context, state types.EvalState, index int, data interface{}) error
}

// DatasetService defines operations for dataset management
type DatasetService interface {
	// GetDatasetByID retrieves QA pairs from dataset by ID
	GetDatasetByID(ctx context.Context, datasetID string) ([]*types.QAPair, error)
	GetEvaluationDataset(ctx context.Context, datasetID string) (*types.EvaluationDataset, error)
	GetEvaluationDatasetVersion(ctx context.Context, datasetID string, version string) (*types.EvaluationDataset, error)
	ListEvaluationDatasets(ctx context.Context) ([]types.EvaluationDatasetSummary, error)
	CreateEvaluationDataset(ctx context.Context, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error)
	GetEvaluationDatasetDraft(ctx context.Context, datasetID string) (*types.EvaluationDatasetDraft, error)
	SaveEvaluationDatasetDraft(ctx context.Context, datasetID string, revision int, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error)
	PublishEvaluationDataset(ctx context.Context, datasetID string, revision int) (*types.EvaluationDatasetVersion, error)
	ListEvaluationDatasetVersions(ctx context.Context, datasetID string) ([]types.EvaluationDatasetVersion, error)
}
