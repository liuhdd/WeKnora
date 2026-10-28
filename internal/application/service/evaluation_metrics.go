package service

import (
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

// evaluationRetrievalIDs 对完整语料匹配并保留未命中项 / Matches the full corpus while retaining irrelevant results.
// 未匹配段落用负编号参与精确率及排序计算 / Unmatched passages use negative IDs in precision and ranking calculations.
func evaluationRetrievalIDs(documents []types.PipelineTraceDocument, corpus []types.EvaluationPassage) []int {
	ids := make([]int, 0, len(documents))
	seen := make(map[int]struct{})
	for index, document := range documents {
		id := -index - 1
		if document.Content != "" {
			for _, passage := range corpus {
				if passage.Text == document.Content {
					id = passage.ID
					break
				}
			}
			if id < 0 {
				for _, passage := range corpus {
					if passage.Text != "" && (strings.Contains(passage.Text, document.Content) || strings.Contains(document.Content, passage.Text)) {
						id = passage.ID
						break
					}
				}
			}
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func calculateEvaluationMetrics(item types.EvaluationDatasetItem, documents []types.PipelineTraceDocument, answer string, corpus []types.EvaluationPassage) *types.MetricResult {
	input := &types.MetricInput{
		RetrievalGT: [][]int{item.PIDs}, RetrievalIDs: evaluationRetrievalIDs(documents, corpus),
		GeneratedTexts: answer, GeneratedGT: item.Answer,
	}
	result := &types.MetricResult{}
	for _, calculator := range metricCalculators {
		*calculator.getField(result) = calculator.calc.Compute(input)
	}
	return result
}

// averageEvaluationResults 仅汇总成功题目，失败数单独报告 / Aggregates successful questions and reports failures separately.
func averageEvaluationResults(results []types.EvaluationItemResult) *types.MetricResult {
	metrics := &MetricList{}
	for _, result := range results {
		if result.Status == "success" && result.Metric != nil {
			metrics.results = append(metrics.results, result.Metric)
		}
	}
	if len(metrics.results) == 0 {
		return nil
	}
	return metrics.Avg()
}
