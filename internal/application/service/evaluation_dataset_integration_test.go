package service

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/require"
)

func TestEvaluationDatasetReadsRealParquetAndStableSnapshot(t *testing.T) {
	data, err := loadEvaluationDataset(filepath.Join("..", "..", "..", "dataset", "samples"))
	require.NoError(t, err)
	first, err := evaluationDatasetSnapshot(data)
	require.NoError(t, err)
	second, err := evaluationDatasetSnapshot(data)
	require.NoError(t, err)
	require.Equal(t, first.Version, second.Version)
	require.Len(t, first.Version, 64)
	require.Equal(t, len(data.corpus), len(first.Corpus))
	require.NotEmpty(t, first.Items)
	for i := 1; i < len(first.Items); i++ {
		require.Less(t, first.Items[i-1].QID, first.Items[i].QID)
	}
}

func TestEvaluationDatasetRejectsBrokenEvidenceReference(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "queries.parquet"), []TextInfo{{ID: 1, Text: "How long?"}}))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "corpus.parquet"), []TextInfo{{ID: 3, Text: "Seven days"}}))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "answers.parquet"), []TextInfo{{ID: 5, Text: "Seven days"}}))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "qas.parquet"), []QaInfo{{QID: 1, AID: 5}}))
	require.NoError(t, parquet.WriteFile(filepath.Join(dir, "qrels.parquet"), []RelsInfo{{QID: 1, PID: 404}}))
	_, err := loadEvaluationDataset(dir)
	require.ErrorContains(t, err, "question 1 references missing passage 404")
}

func TestEvaluationScoresRetainIrrelevantRetrievedPassages(t *testing.T) {
	item := types.EvaluationDatasetItem{QID: 1, Question: "Retention?", Answer: "Seven days", PIDs: []int{3}}
	corpus := []types.EvaluationPassage{{ID: 3, Text: "Backups are kept for seven days."}, {ID: 9, Text: "Exports use CSV."}}
	documents := []types.PipelineTraceDocument{
		{ID: "wrong", Content: "Exports use CSV.", Score: 0.99},
		{ID: "right", Content: "Backups are kept for seven days.", Score: 0.9},
	}
	result := calculateEvaluationMetrics(item, documents, "Seven days", corpus)
	require.InDelta(t, 0.5, result.RetrievalMetrics.Precision, 1e-6)
	require.InDelta(t, 1, result.RetrievalMetrics.Recall, 1e-6)
	require.InDelta(t, 0.5, result.RetrievalMetrics.MRR, 1e-6)
	unknown := append([]types.PipelineTraceDocument{{ID: "unknown", Content: "Unmatched text"}}, documents...)
	unknownScore := calculateEvaluationMetrics(item, unknown, "Seven days", corpus)
	require.InDelta(t, 1.0/3, unknownScore.RetrievalMetrics.Precision, 1e-6)
	require.InDelta(t, 1.0/3, unknownScore.RetrievalMetrics.MRR, 1e-6)
}

func TestEvaluationRankingScoresPenalizeMissingEvidence(t *testing.T) {
	item := types.EvaluationDatasetItem{QID: 1, Answer: "Seven days", PIDs: []int{3, 9}}
	corpus := []types.EvaluationPassage{{ID: 3, Text: "Retain backups for seven days."}, {ID: 9, Text: "Only administrators may change retention."}}
	documents := []types.PipelineTraceDocument{{ID: "only-first", Content: corpus[0].Text}}
	result := calculateEvaluationMetrics(item, documents, "Seven days", corpus)
	require.InDelta(t, 0.5, result.RetrievalMetrics.Recall, 1e-6)
	require.InDelta(t, 0.5, result.RetrievalMetrics.MAP, 1e-6)
	idealDCG := 1 + 1/math.Log2(3)
	require.InDelta(t, 1/idealDCG, result.RetrievalMetrics.NDCG3, 1e-6)
	require.InDelta(t, 1/idealDCG, result.RetrievalMetrics.NDCG10, 1e-6)
}

func TestEvaluationAcceptsIndexedCorpusWhileEnrichmentRuns(t *testing.T) {
	knowledge := &types.Knowledge{ID: "indexed-corpus", ParseStatus: types.ParseStatusProcessing}
	// 运行实际索引完成状态转换，而非手造就绪状态 / Run the real indexing lifecycle transition instead of constructing a ready fixture.
	finalizeIndexedKnowledgeState(knowledge, 4096, 2, false, time.Now())
	require.Equal(t, types.ParseStatusProcessing, knowledge.ParseStatus)
	require.NoError(t, validateEvaluationCorpusIndex(knowledge))
	knowledge.ParseStatus = types.ParseStatusFailed
	knowledge.ErrorMessage = "Embedding provider rejected request"
	require.ErrorContains(t, validateEvaluationCorpusIndex(knowledge), "Embedding provider rejected request")
}

func TestEvaluationAggregationKeepsFailedQuestionsUnscored(t *testing.T) {
	failed := types.EvaluationItemResult{QID: 1, Status: "failed", Error: "Provider rejected request"}
	require.Nil(t, averageEvaluationResults([]types.EvaluationItemResult{failed}))
	item := types.EvaluationDatasetItem{QID: 2, Answer: "Seven days", PIDs: []int{3}}
	corpus := []types.EvaluationPassage{{ID: 3, Text: "Retain backups for seven days."}}
	metric := calculateEvaluationMetrics(item, []types.PipelineTraceDocument{{ID: "right", Content: corpus[0].Text}}, "Seven days", corpus)
	combined := averageEvaluationResults([]types.EvaluationItemResult{failed, {QID: 2, Status: "success", Metric: metric}})
	require.Equal(t, metric, combined)
}
