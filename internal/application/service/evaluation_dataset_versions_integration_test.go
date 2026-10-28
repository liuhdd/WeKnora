package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newManagedEvaluationDatasetService(t *testing.T) (*DatasetService, *gorm.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "datasets.sqlite")
	db, err := gorm.Open(sqlite.Open(path+"?_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{})
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "sqlite", "000052_evaluation_datasets.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	return &DatasetService{repository: repository.NewEvaluationDatasetRepository(db)}, db
}

func managedEvaluationContent(answer string) types.EvaluationDatasetContent {
	return types.EvaluationDatasetContent{
		Questions: []types.EvaluationDatasetQuestion{{QID: 0, Question: "Retention?", Answer: answer, PIDs: []int{0}}},
		Corpus:    []types.EvaluationPassage{{ID: 0, Text: "  Backups are retained for seven days.\nKeep this formatting.  "}, {ID: 5, Text: "Unrelated export guidance remains in the corpus."}},
	}
}

func requireDatasetHTTPCode(t *testing.T, err error, code int) {
	t.Helper()
	require.Error(t, err)
	app, ok := apperrors.IsAppError(err)
	require.True(t, ok, err)
	require.Equal(t, code, app.HTTPCode)
}

func requireSameDatasetDraft(t *testing.T, expected *types.EvaluationDatasetDraft, actual *types.EvaluationDatasetDraft) {
	t.Helper()
	require.Equal(t, expected.ID, actual.ID)
	require.Equal(t, expected.Name, actual.Name)
	require.Equal(t, expected.Description, actual.Description)
	require.Equal(t, expected.Revision, actual.Revision)
	require.Equal(t, expected.Content, actual.Content)
	require.True(t, expected.UpdatedAt.Equal(actual.UpdatedAt))
}

func TestEvaluationDatasetVersionsFreezeFullCorpusAndRestoreDraft(t *testing.T) {
	service, db := newManagedEvaluationDatasetService(t)
	ctx := types.WithExecutionTenant(context.Background(), 1)
	blank := types.EvaluationDatasetContent{Questions: []types.EvaluationDatasetQuestion{}, Corpus: []types.EvaluationPassage{}}
	draft, err := service.CreateEvaluationDataset(ctx, "Backups", "Draft", blank)
	require.NoError(t, err)
	_, err = service.GetEvaluationDataset(ctx, draft.ID)
	requireDatasetHTTPCode(t, err, 404)
	_, err = service.PublishEvaluationDataset(ctx, draft.ID, draft.Revision)
	requireDatasetHTTPCode(t, err, 400)
	content := managedEvaluationContent("Seven days")
	saved, err := service.SaveEvaluationDatasetDraft(ctx, draft.ID, draft.Revision, "Backups", "First", content)
	require.NoError(t, err)
	version1, err := service.PublishEvaluationDataset(ctx, draft.ID, saved.Revision)
	require.NoError(t, err)
	require.Equal(t, "v1", version1.Version)
	require.Len(t, version1.ContentHash, 64)
	_, err = service.PublishEvaluationDataset(ctx, draft.ID, saved.Revision)
	requireDatasetHTTPCode(t, err, 409)
	draft, err = service.GetEvaluationDatasetDraft(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, saved.Revision+1, draft.Revision)
	saved, err = service.SaveEvaluationDatasetDraft(ctx, draft.ID, draft.Revision, "Backups v2", "Second", managedEvaluationContent("Updated answer"))
	require.NoError(t, err)
	version2, err := service.PublishEvaluationDataset(ctx, draft.ID, saved.Revision)
	require.NoError(t, err)
	require.Equal(t, "v2", version2.Version)
	require.NotEqual(t, version1.ContentHash, version2.ContentHash)
	// 新实例从数据库读取，旧版仍保留完整干扰语料和原文空白 / A fresh service reads the immutable version with distractors and whitespace intact.
	restarted := &DatasetService{repository: repository.NewEvaluationDatasetRepository(db)}
	old, err := restarted.GetEvaluationDatasetVersion(ctx, draft.ID, "v1")
	require.NoError(t, err)
	require.Equal(t, "Seven days", old.Items[0].Answer)
	require.Equal(t, content.Corpus, old.Corpus)
	require.Equal(t, version1.ContentHash, old.Version)
	require.Equal(t, "v1", old.PublishedVersion)
	currentDraft, err := restarted.GetEvaluationDatasetDraft(ctx, draft.ID)
	require.NoError(t, err)
	_, err = restarted.SaveEvaluationDatasetDraft(ctx, draft.ID, currentDraft.Revision, "Restored draft", "Restored from v1", content)
	require.NoError(t, err)
	latest, err := restarted.GetEvaluationDataset(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "v2", latest.PublishedVersion)
	require.Equal(t, "Updated answer", latest.Items[0].Answer)
	versions, err := restarted.ListEvaluationDatasetVersions(ctx, draft.ID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	require.Equal(t, "v2", versions[0].Version)
	_, err = restarted.GetEvaluationDatasetVersion(ctx, draft.ID, "v99")
	requireDatasetHTTPCode(t, err, 404)
}

func TestEvaluationDatasetConcurrentSaveAndPublishUseRevision(t *testing.T) {
	service, _ := newManagedEvaluationDatasetService(t)
	ctx := types.WithExecutionTenant(context.Background(), 1)
	draft, err := service.CreateEvaluationDataset(ctx, "Concurrent", "", managedEvaluationContent("Seven days"))
	require.NoError(t, err)
	var group sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for index := 0; index < 2; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := service.SaveEvaluationDatasetDraft(ctx, draft.ID, draft.Revision, draft.Name, draft.Description, draft.Content)
			results <- err
		}()
	}
	close(start)
	group.Wait()
	first, second := <-results, <-results
	if first == nil {
		requireDatasetHTTPCode(t, second, 409)
	} else {
		require.NoError(t, second)
		requireDatasetHTTPCode(t, first, 409)
	}
	fresh, err := service.GetEvaluationDatasetDraft(ctx, draft.ID)
	require.NoError(t, err)
	start = make(chan struct{})
	for index := 0; index < 2; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := service.PublishEvaluationDataset(ctx, fresh.ID, fresh.Revision)
			results <- err
		}()
	}
	close(start)
	group.Wait()
	first, second = <-results, <-results
	if first == nil {
		requireDatasetHTTPCode(t, second, 409)
	} else {
		require.NoError(t, second)
		requireDatasetHTTPCode(t, first, 409)
	}
	versions, err := service.ListEvaluationDatasetVersions(ctx, draft.ID)
	require.NoError(t, err)
	require.Len(t, versions, 1)
}

func TestEvaluationDatasetRejectsInvalidWritesAndScopesTenant(t *testing.T) {
	service, db := newManagedEvaluationDatasetService(t)
	ctx := types.WithExecutionTenant(context.Background(), 1)
	other := types.WithExecutionTenant(context.Background(), 2)
	draft, err := service.CreateEvaluationDataset(ctx, "Scoped", "", managedEvaluationContent("Seven days"))
	require.NoError(t, err)
	_, err = service.GetEvaluationDatasetDraft(other, draft.ID)
	requireDatasetHTTPCode(t, err, 404)
	_, err = service.SaveEvaluationDatasetDraft(other, draft.ID, draft.Revision, draft.Name, draft.Description, draft.Content)
	requireDatasetHTTPCode(t, err, 404)
	_, err = service.PublishEvaluationDataset(other, draft.ID, draft.Revision)
	requireDatasetHTTPCode(t, err, 404)
	broken := managedEvaluationContent("Seven days")
	broken.Questions[0].PIDs = []int{99}
	_, err = service.SaveEvaluationDatasetDraft(ctx, draft.ID, draft.Revision, draft.Name, draft.Description, broken)
	requireDatasetHTTPCode(t, err, 400)
	still, err := service.GetEvaluationDatasetDraft(ctx, draft.ID)
	require.NoError(t, err)
	requireSameDatasetDraft(t, draft, still)
	_, err = service.SaveEvaluationDatasetDraft(ctx, "default", 1, draft.Name, draft.Description, draft.Content)
	requireDatasetHTTPCode(t, err, 403)
	duplicate := managedEvaluationContent("Seven days")
	duplicate.Corpus = append(duplicate.Corpus, types.EvaluationPassage{ID: 6, Text: duplicate.Corpus[0].Text})
	duplicated, err := service.SaveEvaluationDatasetDraft(ctx, draft.ID, draft.Revision, draft.Name, draft.Description, duplicate)
	require.NoError(t, err)
	_, err = service.PublishEvaluationDataset(ctx, draft.ID, duplicated.Revision)
	requireDatasetHTTPCode(t, err, 400)
	var count int64
	require.NoError(t, db.Model(&types.EvaluationDatasetVersionRecord{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestEvaluationDatasetPublicationBoundsSharedEvidenceExpansion(t *testing.T) {
	service, db := newManagedEvaluationDatasetService(t)
	ctx := types.WithExecutionTenant(context.Background(), 1)
	content := types.EvaluationDatasetContent{
		Questions: make([]types.EvaluationDatasetQuestion, 129),
		Corpus: []types.EvaluationPassage{{
			ID: 0, Text: strings.Repeat("a", types.EvaluationDatasetMaxTextBytes),
		}},
	}
	for index := range content.Questions {
		content.Questions[index] = types.EvaluationDatasetQuestion{
			QID: index, Question: "Find the evidence", Answer: "a", PIDs: []int{0},
		}
	}
	draft, err := service.CreateEvaluationDataset(ctx, "Shared evidence", "", content)
	require.NoError(t, err)
	_, err = service.PublishEvaluationDataset(ctx, draft.ID, draft.Revision)
	requireDatasetHTTPCode(t, err, 400)
	require.ErrorContains(t, err, "total referenced evidence beyond 8 MiB")
	unchanged, err := service.GetEvaluationDatasetDraft(ctx, draft.ID)
	require.NoError(t, err)
	requireSameDatasetDraft(t, draft, unchanged)
	var count int64
	require.NoError(t, db.Model(&types.EvaluationDatasetVersionRecord{}).Count(&count).Error)
	require.Zero(t, count)
	// 共享引用的精确边界可发布，超出后仍保留完整草稿。
	// The exact shared-reference budget is publishable; excess references remain safely in the draft.
	content.Questions = content.Questions[:128]
	saved, err := service.SaveEvaluationDatasetDraft(ctx, draft.ID, draft.Revision, draft.Name, draft.Description, content)
	require.NoError(t, err)
	version, err := service.PublishEvaluationDataset(ctx, draft.ID, saved.Revision)
	require.NoError(t, err)
	require.Equal(t, "v1", version.Version)
}

func TestEvaluationDatasetPublishRollbackPreservesDraftOnStorageFailure(t *testing.T) {
	service, db := newManagedEvaluationDatasetService(t)
	ctx := types.WithExecutionTenant(context.Background(), 1)
	draft, err := service.CreateEvaluationDataset(ctx, "Atomic", "", managedEvaluationContent("Seven days"))
	require.NoError(t, err)
	// 通过真实SQLite约束拒绝版本写入，验证事务回滚 / A real SQLite constraint rejects the version insert to verify rollback.
	require.NoError(t, db.Exec("CREATE TRIGGER reject_dataset_version BEFORE INSERT ON evaluation_dataset_versions BEGIN SELECT RAISE(ABORT, 'version storage rejected'); END;").Error)
	_, err = service.PublishEvaluationDataset(ctx, draft.ID, draft.Revision)
	require.ErrorContains(t, err, "version storage rejected")
	unchanged, err := service.GetEvaluationDatasetDraft(ctx, draft.ID)
	require.NoError(t, err)
	requireSameDatasetDraft(t, draft, unchanged)
	_, err = service.GetEvaluationDataset(ctx, draft.ID)
	requireDatasetHTTPCode(t, err, 404)
}
