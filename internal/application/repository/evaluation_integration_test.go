package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openEvaluationDatabase 使用真实SQLite迁移及文件 / Uses the real SQLite migration and database file.
func openEvaluationDatabase(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "sqlite", "000051_evaluation_runs.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(sql)).Error)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	return db
}

func TestEvaluationHistoryPersistsAndScopesTenants(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evaluation.sqlite")
	db := openEvaluationDatabase(t, path)
	repo := NewEvaluationRepository(db)
	ctx := context.Background()
	for _, tenant := range []uint64{1, 2} {
		id := "run-1"
		if tenant == 2 {
			id = "run-2"
		}
		detail := &types.EvaluationDetail{
			Task:   &types.EvaluationTask{ID: id, TenantID: tenant, DatasetID: "default", DatasetVersion: "snapshot-v1", MetricVersion: "corpus-passage-v2", Status: types.EvaluationStatueSuccess, StartTime: time.Now()},
			Params: &types.ChatManage{PipelineRequest: types.PipelineRequest{ChatModelID: "chat-v1"}},
			Results: []types.EvaluationItemResult{{
				QID: 7, Question: "How long?", ReferenceAnswer: "Seven days", Answer: "Seven days", Status: "success",
				PIDs: []int{3}, Passages: []string{"Retain backups for seven days."},
				Steps: []types.PipelineTraceStep{{ID: "chunk_search", Name: "chunk_search", Status: "success", Documents: []types.PipelineTraceDocument{{ID: "chunk-3", Content: "Retain backups for seven days.", Score: 0.9}}}},
			}},
		}
		require.NoError(t, repo.Save(ctx, detail))
	}
	// 新连接模拟服务重启后读取 / A fresh connection simulates reading after restart.
	reopened := openEvaluationDatabase(t, path)
	restarted := NewEvaluationRepository(reopened)
	detail, err := restarted.Get(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Equal(t, "snapshot-v1", detail.Task.DatasetVersion)
	require.Equal(t, "Seven days", detail.Results[0].Answer)
	require.Equal(t, []string{"Retain backups for seven days."}, detail.Results[0].Passages)
	require.Equal(t, 0.9, detail.Results[0].Steps[0].Documents[0].Score)
	_, err = restarted.Get(ctx, 2, "run-1")
	require.Error(t, err)
	page, err := restarted.List(ctx, 1, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Nil(t, page.Items[0].Results)
	require.Equal(t, "run-1", page.Items[0].Task.ID)
}

func TestEvaluationExpiredLeaseDoesNotCloseHealthyRunsOrReopen(t *testing.T) {
	db := openEvaluationDatabase(t, filepath.Join(t.TempDir(), "leases.sqlite"))
	repo := NewEvaluationRepository(db)
	ctx := context.Background()
	for _, id := range []string{"expired", "healthy"} {
		require.NoError(t, repo.Save(ctx, &types.EvaluationDetail{
			Task:    &types.EvaluationTask{ID: id, TenantID: 1, DatasetID: "default", StartTime: time.Now(), Status: types.EvaluationStatueRunning, Total: 1},
			Results: []types.EvaluationItemResult{{QID: 1, Status: "running"}},
		}))
	}
	require.NoError(t, db.Model(&types.EvaluationRunRecord{}).Where("id = ?", "expired").Update("heartbeat_at", time.Now().Add(-2*time.Minute)).Error)
	expired, err := repo.Get(ctx, 1, "expired")
	require.NoError(t, err)
	require.Equal(t, types.EvaluationStatueFailed, expired.Task.Status)
	require.Contains(t, expired.Task.ErrMsg, "interrupted")
	require.Equal(t, "failed", expired.Results[0].Status)
	require.Equal(t, 1, expired.Task.Failed)
	require.NotNil(t, expired.Task.EndTime)
	require.Error(t, repo.Heartbeat(ctx, 1, "expired"))
	expired.Task.Status = types.EvaluationStatueRunning
	require.Error(t, repo.Save(ctx, expired))
	healthy, err := repo.Get(ctx, 1, "healthy")
	require.NoError(t, err)
	require.Equal(t, types.EvaluationStatueRunning, healthy.Task.Status)
	require.NoError(t, repo.Heartbeat(ctx, 1, "healthy"))
	require.Error(t, repo.Heartbeat(ctx, 2, "healthy"))
}
