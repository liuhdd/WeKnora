package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type evaluationDatasetRepository struct{ db *gorm.DB }

func NewEvaluationDatasetRepository(db *gorm.DB) interfaces.EvaluationDatasetRepository {
	return &evaluationDatasetRepository{db: db}
}

func (r *evaluationDatasetRepository) Create(ctx context.Context, tenantID uint64, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error) {
	payload, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("encode new dataset draft: %w", err)
	}
	now := time.Now()
	row := types.EvaluationDatasetRecord{
		ID: uuid.NewString(), TenantID: tenantID, Name: name, Description: description,
		DraftRevision: 1, DraftContent: string(payload), DraftTotal: len(content.Questions), CreatedAt: now, UpdatedAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create evaluation dataset: %w", err)
	}
	return evaluationDraftFromRecord(row)
}

func evaluationDatasetRecord(ctx context.Context, db *gorm.DB, tenantID uint64, datasetID string) (*types.EvaluationDatasetRecord, error) {
	var row types.EvaluationDatasetRecord
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, datasetID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("Evaluation dataset not found")
		}
		return nil, fmt.Errorf("read evaluation dataset %s: %w", datasetID, err)
	}
	return &row, nil
}

func evaluationDraftFromRecord(row types.EvaluationDatasetRecord) (*types.EvaluationDatasetDraft, error) {
	var content types.EvaluationDatasetContent
	if err := json.Unmarshal([]byte(row.DraftContent), &content); err != nil {
		return nil, fmt.Errorf("decode dataset %s draft: %w", row.ID, err)
	}
	return &types.EvaluationDatasetDraft{ID: row.ID, Name: row.Name, Description: row.Description, Revision: row.DraftRevision, Content: content, UpdatedAt: row.UpdatedAt}, nil
}

func (r *evaluationDatasetRepository) GetDraft(ctx context.Context, tenantID uint64, datasetID string) (*types.EvaluationDatasetDraft, error) {
	row, err := evaluationDatasetRecord(ctx, r.db, tenantID, datasetID)
	if err != nil {
		return nil, err
	}
	return evaluationDraftFromRecord(*row)
}

func (r *evaluationDatasetRepository) SaveDraft(ctx context.Context, tenantID uint64, datasetID string, revision int, name string, description string, content types.EvaluationDatasetContent) (*types.EvaluationDatasetDraft, error) {
	payload, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("encode dataset %s draft: %w", datasetID, err)
	}
	var draft *types.EvaluationDatasetDraft
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&types.EvaluationDatasetRecord{}).
			Where("tenant_id = ? AND id = ? AND draft_revision = ?", tenantID, datasetID, revision).
			Updates(map[string]interface{}{
				"name": name, "description": description, "draft_content": string(payload), "draft_total": len(content.Questions),
				"draft_revision": gorm.Expr("draft_revision + 1"), "updated_at": time.Now(),
			})
		if result.Error != nil {
			return fmt.Errorf("save dataset %s draft: %w", datasetID, result.Error)
		}
		row, err := evaluationDatasetRecord(ctx, tx, tenantID, datasetID)
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return apperrors.NewConflictError("Dataset draft changed; reload it before saving")
		}
		draft, err = evaluationDraftFromRecord(*row)
		return err
	})
	return draft, err
}

func evaluationVersionFromRecord(row types.EvaluationDatasetVersionRecord) types.EvaluationDatasetVersion {
	return types.EvaluationDatasetVersion{
		Version: fmt.Sprintf("v%d", row.Version), ContentHash: row.ContentHash, Total: row.Total,
		CreatedAt: &row.CreatedAt, Name: row.Name, Description: row.Description,
	}
}

// Publish 在同一事务中抢占草稿revision并插入不可变版本 / Claims the draft revision and inserts an immutable version in one transaction.
func (r *evaluationDatasetRepository) Publish(ctx context.Context, tenantID uint64, datasetID string, revision int) (*types.EvaluationDatasetVersion, error) {
	var version *types.EvaluationDatasetVersion
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		result := tx.Model(&types.EvaluationDatasetRecord{}).
			Where("tenant_id = ? AND id = ? AND draft_revision = ?", tenantID, datasetID, revision).
			Updates(map[string]interface{}{"draft_revision": gorm.Expr("draft_revision + 1"), "latest_version": gorm.Expr("latest_version + 1"), "updated_at": now})
		if result.Error != nil {
			return fmt.Errorf("publish dataset %s: %w", datasetID, result.Error)
		}
		row, err := evaluationDatasetRecord(ctx, tx, tenantID, datasetID)
		if err != nil {
			return err
		}
		if result.RowsAffected != 1 {
			return apperrors.NewConflictError("Dataset draft changed; reload it before publishing")
		}
		hash := sha256.Sum256([]byte(row.DraftContent))
		published := types.EvaluationDatasetVersionRecord{
			DatasetID: datasetID, TenantID: tenantID, Version: row.LatestVersion, Name: row.Name, Description: row.Description,
			Content: row.DraftContent, ContentHash: hex.EncodeToString(hash[:]), Total: row.DraftTotal, CreatedAt: now,
		}
		if err := tx.Create(&published).Error; err != nil {
			return fmt.Errorf("insert dataset %s published version: %w", datasetID, err)
		}
		if err := tx.Model(&types.EvaluationDatasetRecord{}).Where("tenant_id = ? AND id = ?", tenantID, datasetID).
			Updates(map[string]interface{}{"published_hash": published.ContentHash, "published_total": published.Total}).Error; err != nil {
			return fmt.Errorf("update dataset %s published pointer: %w", datasetID, err)
		}
		value := evaluationVersionFromRecord(published)
		version = &value
		return nil
	})
	return version, err
}

func (r *evaluationDatasetRepository) GetPublished(ctx context.Context, tenantID uint64, datasetID string, version int) (*types.EvaluationPublishedDataset, error) {
	if version == 0 {
		row, err := evaluationDatasetRecord(ctx, r.db, tenantID, datasetID)
		if err != nil {
			return nil, err
		}
		version = row.LatestVersion
	}
	if version == 0 {
		return nil, apperrors.NewNotFoundError("Dataset has no published version; publish its draft before evaluating")
	}
	var row types.EvaluationDatasetVersionRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND dataset_id = ? AND version = ?", tenantID, datasetID, version).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("Published dataset version not found")
		}
		return nil, fmt.Errorf("read dataset %s version v%d: %w", datasetID, version, err)
	}
	var content types.EvaluationDatasetContent
	if err := json.Unmarshal([]byte(row.Content), &content); err != nil {
		return nil, fmt.Errorf("decode dataset %s version v%d: %w", datasetID, version, err)
	}
	return &types.EvaluationPublishedDataset{ID: datasetID, EvaluationDatasetVersion: evaluationVersionFromRecord(row), Content: content}, nil
}

func (r *evaluationDatasetRepository) ListVersions(ctx context.Context, tenantID uint64, datasetID string) ([]types.EvaluationDatasetVersion, error) {
	if _, err := evaluationDatasetRecord(ctx, r.db, tenantID, datasetID); err != nil {
		return nil, err
	}
	var rows []types.EvaluationDatasetVersionRecord
	if err := r.db.WithContext(ctx).Select("dataset_id", "version", "content_hash", "total", "created_at", "name", "description").
		Where("tenant_id = ? AND dataset_id = ?", tenantID, datasetID).Order("version DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list dataset %s versions: %w", datasetID, err)
	}
	versions := make([]types.EvaluationDatasetVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, evaluationVersionFromRecord(row))
	}
	return versions, nil
}

func (r *evaluationDatasetRepository) List(ctx context.Context, tenantID uint64) ([]types.EvaluationDatasetSummary, error) {
	var rows []types.EvaluationDatasetRecord
	if err := r.db.WithContext(ctx).Omit("draft_content").Where("tenant_id = ?", tenantID).Order("updated_at DESC, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list evaluation datasets: %w", err)
	}
	result := make([]types.EvaluationDatasetSummary, 0, len(rows))
	for _, row := range rows {
		version := ""
		total := row.DraftTotal
		if row.LatestVersion > 0 {
			version = fmt.Sprintf("v%d", row.LatestVersion)
			total = row.PublishedTotal
		}
		result = append(result, types.EvaluationDatasetSummary{ID: row.ID, Name: row.Name, Description: row.Description, Version: row.PublishedHash,
			Total: total, DraftRevision: row.DraftRevision, PublishedVersion: version, ReadOnly: false})
	}
	return result, nil
}
