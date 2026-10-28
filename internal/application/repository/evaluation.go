package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type evaluationRepository struct{ db *gorm.DB }

// NewEvaluationRepository 创建数据库存储 / Creates the database-backed evaluation store.
func NewEvaluationRepository(db *gorm.DB) interfaces.EvaluationRepository {
	return &evaluationRepository{db: db}
}

func (r *evaluationRepository) Save(ctx context.Context, detail *types.EvaluationDetail) error {
	if detail == nil || detail.Task == nil || detail.Task.ID == "" || detail.Task.TenantID == 0 {
		return fmt.Errorf("save evaluation: task ID and tenant ID are required")
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode evaluation %s: %w", detail.Task.ID, err)
	}
	summary := *detail
	summary.Results = nil
	summaryPayload, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("encode evaluation summary %s: %w", detail.Task.ID, err)
	}
	row := types.EvaluationRunRecord{
		ID: detail.Task.ID, TenantID: detail.Task.TenantID, DatasetID: detail.Task.DatasetID,
		Status: detail.Task.Status, StartedAt: detail.Task.StartTime,
		HeartbeatAt: time.Now(),
		Detail:      string(payload), Summary: string(summaryPayload),
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "detail", "summary", "heartbeat_at"}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Table: "evaluation_runs", Name: "tenant_id"}, Value: row.TenantID},
			clause.IN{Column: clause.Column{Table: "evaluation_runs", Name: "status"}, Values: []interface{}{types.EvaluationStatuePending, types.EvaluationStatueRunning}},
		}},
	}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("save evaluation %s: %w", row.ID, result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.NewConflictError("Evaluation task is already complete or belongs to another workspace")
	}
	return nil
}

func (r *evaluationRepository) Get(ctx context.Context, tenantID uint64, taskID string) (*types.EvaluationDetail, error) {
	var row types.EvaluationRunRecord
	if err := r.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, taskID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("Evaluation task not found")
		}
		return nil, fmt.Errorf("read evaluation %s: %w", taskID, err)
	}
	if evaluationLeaseExpired(row, time.Now()) {
		if err := r.expireRun(ctx, row); err != nil {
			return nil, err
		}
		return r.Get(ctx, tenantID, taskID)
	}
	var detail types.EvaluationDetail
	if err := json.Unmarshal([]byte(row.Detail), &detail); err != nil {
		return nil, fmt.Errorf("decode evaluation %s: %w", taskID, err)
	}
	return &detail, nil
}

func (r *evaluationRepository) List(ctx context.Context, tenantID uint64, page int, pageSize int) (*types.EvaluationRunPage, error) {
	result := &types.EvaluationRunPage{Items: make([]*types.EvaluationDetail, 0), Page: page, PageSize: pageSize}
	query := r.db.WithContext(ctx).Model(&types.EvaluationRunRecord{}).Where("tenant_id = ?", tenantID)
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, fmt.Errorf("count evaluation history: %w", err)
	}
	var rows []types.EvaluationRunRecord
	if err := query.Select("id", "summary", "status", "heartbeat_at").Order("started_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list evaluation history: %w", err)
	}
	for _, row := range rows {
		if evaluationLeaseExpired(row, time.Now()) {
			detail, err := r.Get(ctx, tenantID, row.ID)
			if err != nil {
				return nil, err
			}
			detail.Results = nil
			result.Items = append(result.Items, detail)
			continue
		}
		var detail types.EvaluationDetail
		if err := json.Unmarshal([]byte(row.Summary), &detail); err != nil {
			return nil, fmt.Errorf("decode evaluation summary %s: %w", row.ID, err)
		}
		result.Items = append(result.Items, &detail)
	}
	return result, nil
}

// Heartbeat 延长单任务租约，终态不可恢复运行 / Renews one run's lease without reopening terminal runs.
func (r *evaluationRepository) Heartbeat(ctx context.Context, tenantID uint64, taskID string) error {
	result := r.db.WithContext(ctx).Model(&types.EvaluationRunRecord{}).
		Where("tenant_id = ? AND id = ? AND status IN ?", tenantID, taskID, []types.EvaluationStatue{types.EvaluationStatuePending, types.EvaluationStatueRunning}).
		Update("heartbeat_at", time.Now())
	if result.Error != nil {
		return fmt.Errorf("renew evaluation lease %s: %w", taskID, result.Error)
	}
	if result.RowsAffected == 0 {
		return apperrors.NewConflictError("Evaluation task is no longer active")
	}
	return nil
}

func evaluationLeaseExpired(row types.EvaluationRunRecord, now time.Time) bool {
	return (row.Status == types.EvaluationStatuePending || row.Status == types.EvaluationStatueRunning) &&
		row.HeartbeatAt.Before(now.Add(-90*time.Second))
}

// expireRun 用条件更新只关闭已过期任务 / Closes only the expired run with a conditional update.
func (r *evaluationRepository) expireRun(ctx context.Context, row types.EvaluationRunRecord) error {
	var detail types.EvaluationDetail
	if err := json.Unmarshal([]byte(row.Detail), &detail); err != nil {
		return fmt.Errorf("decode expired evaluation %s: %w", row.ID, err)
	}
	now := time.Now()
	detail.Task.Status = types.EvaluationStatueFailed
	detail.Task.EndTime = &now
	detail.Task.ErrMsg = "Evaluation worker stopped sending heartbeats for over 90 seconds; this run was interrupted. Start a new evaluation."
	for i := range detail.Results {
		if detail.Results[i].Status == "pending" || detail.Results[i].Status == "running" {
			detail.Results[i].Status = "failed"
			detail.Results[i].Error = detail.Task.ErrMsg
			detail.Task.Failed++
			detail.Task.Finished++
		}
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode interrupted evaluation %s: %w", row.ID, err)
	}
	detail.Results = nil
	summary, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode interrupted evaluation summary %s: %w", row.ID, err)
	}
	result := r.db.WithContext(ctx).Model(&types.EvaluationRunRecord{}).
		Where("tenant_id = ? AND id = ? AND status IN ? AND heartbeat_at < ?", row.TenantID, row.ID,
			[]types.EvaluationStatue{types.EvaluationStatuePending, types.EvaluationStatueRunning}, now.Add(-90*time.Second)).
		Updates(map[string]interface{}{"status": types.EvaluationStatueFailed, "detail": string(payload), "summary": string(summary)})
	if result.Error != nil {
		return fmt.Errorf("close interrupted evaluation %s: %w", row.ID, result.Error)
	}
	return nil
}
