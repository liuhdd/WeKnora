import type { EvaluationDetail, EvaluationMetrics, EvaluationQuestionResult } from '../../api/evaluation'

export const retrievalMetricKeys = ['precision', 'recall', 'ndcg3', 'ndcg10', 'mrr', 'map'] as const
export const generationMetricKeys = ['bleu1', 'bleu2', 'bleu4', 'rouge1', 'rouge2', 'rougel'] as const
export type EvaluationMetricKey = typeof retrievalMetricKeys[number] | typeof generationMetricKeys[number]
export type EvaluationQuestionFilter = 'all' | 'failed' | 'low'

export const metricLabels: Record<EvaluationMetricKey, string> = {
  precision: 'Precision', recall: 'Recall', ndcg3: 'NDCG@3', ndcg10: 'NDCG@10', mrr: 'MRR', map: 'MAP',
  bleu1: 'BLEU-1', bleu2: 'BLEU-2', bleu4: 'BLEU-4', rouge1: 'ROUGE-1', rouge2: 'ROUGE-2', rougel: 'ROUGE-L',
}

export function metricValue(metric: EvaluationMetrics | undefined, key: EvaluationMetricKey): number | undefined {
  if (!metric) return undefined
  if (key in metric.retrieval_metrics) return metric.retrieval_metrics[key as keyof EvaluationMetrics['retrieval_metrics']]
  return metric.generation_metrics[key as keyof EvaluationMetrics['generation_metrics']]
}

export function filterEvaluationQuestions(
  results: EvaluationQuestionResult[], query: string, filter: EvaluationQuestionFilter,
  scoreKey: EvaluationMetricKey, threshold: number,
): EvaluationQuestionResult[] {
  const needle = query.trim().toLocaleLowerCase()
  return results.filter(result => {
    if (needle && !(String(result.qid) + ' ' + result.question + ' ' + result.answer + ' ' + result.reference_answer)
      .toLocaleLowerCase().includes(needle)) return false
    if (filter === 'failed') return result.status === 'failed'
    if (filter === 'low') {
      const score = metricValue(result.metric, scoreKey)
      return result.status === 'success' && score !== undefined && score < threshold
    }
    return true
  })
}

// 数据集版本未知时禁止对比，避免把语料变化归因于参数变化。 Require a known matching version for valid comparisons.
export function canCompareEvaluationRuns(current: EvaluationDetail, baseline: EvaluationDetail): boolean {
  return current.task.id !== baseline.task.id && current.task.dataset_id === baseline.task.dataset_id &&
    !!current.task.dataset_version && current.task.dataset_version === baseline.task.dataset_version &&
    !!current.task.metric_version && current.task.metric_version === baseline.task.metric_version &&
    current.task.status === 2 && baseline.task.status === 2 && !!current.metric && !!baseline.metric
}

export function evaluationDuration(milliseconds: number): string {
  return milliseconds < 1000 ? Math.round(milliseconds) + ' ms' : (milliseconds / 1000).toFixed(2) + ' s'
}

export function evaluationRunDuration(run: EvaluationDetail): number | null {
  if (!run.task.end_time) return null
  const elapsed = Date.parse(run.task.end_time) - Date.parse(run.task.start_time)
  return Number.isFinite(elapsed) && elapsed >= 0 ? elapsed : null
}
