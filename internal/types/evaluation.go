package types

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yanyiwu/gojieba"
)

// Jieba is a global instance of Chinese text segmentation tool
var Jieba *gojieba.Jieba = newJieba()

func newJieba() *gojieba.Jieba {
	dictDir := os.Getenv("JIEBA_DICT_DIR")
	if dictDir == "" {
		return gojieba.NewJieba()
	}

	return gojieba.NewJieba(
		filepath.Join(dictDir, "jieba.dict.utf8"),
		filepath.Join(dictDir, "hmm_model.utf8"),
		filepath.Join(dictDir, "user.dict.utf8"),
		filepath.Join(dictDir, "idf.utf8"),
		filepath.Join(dictDir, "stop_words.utf8"),
	)
}

// EvaluationStatue represents the status of an evaluation task
type EvaluationStatue int

const (
	EvaluationStatuePending EvaluationStatue = iota // Task is waiting to start
	EvaluationStatueRunning                         // Task is in progress
	EvaluationStatueSuccess                         // Task completed successfully
	EvaluationStatueFailed                          // Task failed
)

// EvaluationTask contains information about an evaluation task
type EvaluationTask struct {
	ID                  string `json:"id"`         // Unique task ID
	TenantID            uint64 `json:"tenant_id"`  // Tenant/Organization ID
	DatasetID           string `json:"dataset_id"` // Dataset ID for evaluation
	DatasetVersion      string `json:"dataset_version"`
	DatasetVersionLabel string `json:"dataset_version_label"`
	MetricVersion       string `json:"metric_version"`
	EmbeddingModelID    string `json:"embedding_model_id"`
	KnowledgeBaseID     string `json:"knowledge_base_id,omitempty"`

	StartTime time.Time        `json:"start_time"` // Task start time
	EndTime   *time.Time       `json:"end_time,omitempty"`
	Status    EvaluationStatue `json:"status"`            // Current task status
	ErrMsg    string           `json:"err_msg,omitempty"` // Error message if failed

	Total    int `json:"total,omitempty"`    // Total items to evaluate
	Finished int `json:"finished,omitempty"` // Completed items count
	Failed   int `json:"failed"`
}

// EvaluationDetail contains detailed evaluation information
type EvaluationDetail struct {
	Task    *EvaluationTask        `json:"task"`             // Evaluation task info
	Params  *ChatManage            `json:"params"`           // Evaluation parameters
	Metric  *MetricResult          `json:"metric,omitempty"` // Evaluation metrics
	Results []EvaluationItemResult `json:"results,omitempty"`
}

// EvaluationItemResult 保存逐题答案及执行轨迹 / Stores each question's answer and execution trace.
type EvaluationItemResult struct {
	QID             int                 `json:"qid"`
	Question        string              `json:"question"`
	ReferenceAnswer string              `json:"reference_answer"`
	PIDs            []int               `json:"pids"`
	Passages        []string            `json:"passages"`
	Answer          string              `json:"answer"`
	Status          string              `json:"status"`
	Error           string              `json:"error,omitempty"`
	DurationMS      int64               `json:"duration_ms"`
	Usage           *TokenUsage         `json:"usage,omitempty"`
	Metric          *MetricResult       `json:"metric,omitempty"`
	Steps           []PipelineTraceStep `json:"steps"`
}

// EvaluationDatasetSummary 标识当前数据快照 / Identifies the current dataset snapshot.
type EvaluationDatasetSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Total            int    `json:"total"`
	Description      string `json:"description"`
	ReadOnly         bool   `json:"read_only"`
	DraftRevision    int    `json:"draft_revision"`
	PublishedVersion string `json:"published_version"`
}

// EvaluationDataset 是只读数据集预览 / Is the read-only dataset preview.
type EvaluationDataset struct {
	EvaluationDatasetSummary
	Items  []EvaluationDatasetItem `json:"items"`
	Corpus []EvaluationPassage     `json:"corpus"`
}

// EvaluationDatasetItem 定义问题及人工标注 / Defines a question and its reference labels.
type EvaluationDatasetItem struct {
	QID      int      `json:"qid"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	PIDs     []int    `json:"pids"`
	Passages []string `json:"passages"`
}

// EvaluationPassage 保留语料原始编号 / Retains the corpus passage's original identifier.
type EvaluationPassage struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// EvaluationRunPage 是租户隔离的历史列表 / Is the tenant-scoped history page.
type EvaluationRunPage struct {
	Items    []*EvaluationDetail `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// EvaluationRunRecord 在数据库中保存完整快照 / Persists a complete run snapshot in the database.
type EvaluationRunRecord struct {
	ID          string `gorm:"primaryKey;type:varchar(200)"`
	TenantID    uint64 `gorm:"index:idx_evaluation_runs_tenant_started,priority:1"`
	DatasetID   string `gorm:"type:varchar(100)"`
	Status      EvaluationStatue
	StartedAt   time.Time `gorm:"index:idx_evaluation_runs_tenant_started,priority:2,sort:desc"`
	HeartbeatAt time.Time
	Detail      string `gorm:"type:text"`
	Summary     string `gorm:"type:text"`
}

func (EvaluationRunRecord) TableName() string { return "evaluation_runs" }

// String returns JSON representation of EvaluationTask
func (e *EvaluationTask) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// MetricInput contains input data for metric calculation
type MetricInput struct {
	RetrievalGT  [][]int // Ground truth for retrieval
	RetrievalIDs []int   // Retrieved IDs

	GeneratedTexts string // Generated text for evaluation
	GeneratedGT    string // Ground truth text for comparison
}

// MetricResult contains evaluation metrics
type MetricResult struct {
	RetrievalMetrics  RetrievalMetrics  `json:"retrieval_metrics"`  // Retrieval performance metrics
	GenerationMetrics GenerationMetrics `json:"generation_metrics"` // Text generation quality metrics
}

// RetrievalMetrics contains metrics for retrieval evaluation
type RetrievalMetrics struct {
	Precision float64 `json:"precision"` // Precision score
	Recall    float64 `json:"recall"`    // Recall score

	NDCG3  float64 `json:"ndcg3"`  // Normalized Discounted Cumulative Gain at 3
	NDCG10 float64 `json:"ndcg10"` // Normalized Discounted Cumulative Gain at 10
	MRR    float64 `json:"mrr"`    // Mean Reciprocal Rank
	MAP    float64 `json:"map"`    // Mean Average Precision
}

// GenerationMetrics contains metrics for text generation evaluation
type GenerationMetrics struct {
	BLEU1 float64 `json:"bleu1"` // BLEU-1 score
	BLEU2 float64 `json:"bleu2"` // BLEU-2 score
	BLEU4 float64 `json:"bleu4"` // BLEU-4 score

	ROUGE1 float64 `json:"rouge1"` // ROUGE-1 score
	ROUGE2 float64 `json:"rouge2"` // ROUGE-2 score
	ROUGEL float64 `json:"rougel"` // ROUGE-L score
}

// EvalState represents different stages of evaluation process
type EvalState int

const (
	StateBegin             EvalState = iota // Evaluation started
	StateAfterQaPairs                       // After loading QA pairs
	StateAfterDataset                       // After processing dataset
	StateAfterEmbedding                     // After generating embeddings
	StateAfterVectorSearch                  // After vector search
	StateAfterRerank                        // After reranking
	StateAfterComplete                      // After completion
	StateEnd                                // Evaluation ended
)
