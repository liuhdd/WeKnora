package types

// PipelineTraceDocument 保存阶段结束时的段落与排序分；保留值副本以免后续重排修改历史。
// PipelineTraceDocument snapshots a passage and its ranking score before later stages mutate them.
type PipelineTraceDocument struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Source  string  `json:"source"`
}

// PipelineTraceStep 记录实际执行的阶段，排序分与评测指标分别存储。
// PipelineTraceStep records an executed stage, separating ranking signals from evaluation metrics.
type PipelineTraceStep struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Status      string                  `json:"status"`
	DurationMS  int64                   `json:"duration_ms"`
	Input       string                  `json:"input"`
	Output      string                  `json:"output"`
	Documents   []PipelineTraceDocument `json:"documents"`
	Metric      *MetricResult           `json:"metric,omitempty"`
	MetricScope string                  `json:"metric_scope,omitempty"`
	Error       string                  `json:"error,omitempty"`
}
