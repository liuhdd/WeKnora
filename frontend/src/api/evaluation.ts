import { get, post } from '@/utils/request'

export interface RetrievalMetrics {
  precision: number
  recall: number
  ndcg3: number
  ndcg10: number
  mrr: number
  map: number
}

export interface GenerationMetrics {
  bleu1: number
  bleu2: number
  bleu4: number
  rouge1: number
  rouge2: number
  rougel: number
}

export interface EvaluationMetrics {
  retrieval_metrics: RetrievalMetrics
  generation_metrics: GenerationMetrics
}

export interface EvaluationTask {
  id: string
  dataset_id: string
  dataset_version?: string
  dataset_version_label?: string
  metric_version?: string
  embedding_model_id?: string
  knowledge_base_id?: string
  start_time: string
  end_time?: string
  status: 0 | 1 | 2 | 3
  total?: number
  finished?: number
  failed?: number
  err_msg?: string
}

export interface EvaluationParameters {
  knowledge_base_ids?: string[]
  chat_model_id?: string
  rerank_model_id?: string
  vector_threshold?: number
  keyword_threshold?: number
  embedding_top_k?: number
  rerank_top_k?: number
  rerank_threshold?: number
  enable_rewrite?: boolean
  enable_query_expansion?: boolean
  summary_config?: {
    max_tokens?: number
    top_k?: number
    top_p?: number
    temperature?: number
    prompt?: string
    context_template?: string
  }
}

export interface EvaluationDocument {
  id: string
  content: string
  score: number
  source: string
}

export interface EvaluationStep {
  id: string
  name: string
  status: 'success' | 'failed'
  duration_ms: number
  input: string
  output: string
  documents?: EvaluationDocument[] | null
  metric?: EvaluationMetrics
  metric_scope?: 'retrieval' | 'generation'
  error?: string
}

export interface EvaluationUsage {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

export interface EvaluationQuestionResult {
  qid: number
  question: string
  reference_answer: string
  pids?: number[]
  passages?: string[]
  answer: string
  status: 'pending' | 'running' | 'success' | 'failed'
  error?: string
  duration_ms: number
  usage?: EvaluationUsage
  metric?: EvaluationMetrics
  steps?: EvaluationStep[] | null
}

export interface EvaluationDetail {
  task: EvaluationTask
  params: EvaluationParameters | null
  metric?: EvaluationMetrics
  results?: EvaluationQuestionResult[] | null
}

export interface EvaluationDatasetSummary {
  id: string
  name: string
  version: string
  total: number
  description: string
  published_version: string
  read_only: boolean
  draft_revision: number
}

export interface EvaluationDatasetQuestion {
  qid: number
  question: string
  answer: string
  pids: number[]
  passages: string[]
}

export interface EvaluationDataset extends EvaluationDatasetSummary {
  items: EvaluationDatasetQuestion[]
  corpus: { id: number; text: string }[]
}

export interface EvaluationRunPage {
  items: EvaluationDetail[]
  total: number
  page: number
  page_size: number
}

export interface EvaluationInput {
  dataset_id: string
  dataset_version: string
  knowledge_base_id: string
  chat_id: string
  rerank_id: string
}

interface EvaluationResponse<T> {
  success: boolean
  data: T
}

class EvaluationRequestError extends Error {
  constructor(message: string, options: ErrorOptions) {
    super(message, options)
    this.name = 'EvaluationRequestError'
  }
}

// 保留原始接口错误供诊断，界面展示明确的错误消息。 Preserve API error details while presenting its actionable message.
export async function evaluationRequest<T>(pending: Promise<T>): Promise<T> {
  try {
    return await pending
  } catch (error) {
    if (error !== null && typeof error === 'object' && 'message' in error && typeof error.message === 'string') {
      throw new EvaluationRequestError(error.message, { cause: error })
    }
    throw error
  }
}

// 校验接口必需字段，忽略无关扩展字段。 Validate required API fields and ignore unrelated additions.
function invalidResponse(endpoint: string, field: string): never {
  throw new TypeError('Invalid evaluation response from ' + endpoint + ': ' + field)
}

function validateMetrics(metric: EvaluationMetrics, endpoint: string): void {
  if (!metric?.retrieval_metrics || !metric.generation_metrics) invalidResponse(endpoint, 'metric groups are required')
  const values = [metric.retrieval_metrics.precision, metric.retrieval_metrics.recall,
    metric.retrieval_metrics.ndcg3, metric.retrieval_metrics.ndcg10, metric.retrieval_metrics.mrr,
    metric.retrieval_metrics.map, metric.generation_metrics.bleu1, metric.generation_metrics.bleu2,
    metric.generation_metrics.bleu4, metric.generation_metrics.rouge1, metric.generation_metrics.rouge2,
    metric.generation_metrics.rougel]
  if (!values.every(Number.isFinite)) invalidResponse(endpoint, 'all metric scores must be finite numbers')
}

function validateDetail(detail: EvaluationDetail, endpoint: string): void {
  const task = detail?.task
  if (!task || typeof task.id !== 'string' || typeof task.dataset_id !== 'string' ||
      typeof task.start_time !== 'string' || !Number.isFinite(Date.parse(task.start_time)) || ![0, 1, 2, 3].includes(task.status)) {
    invalidResponse(endpoint, 'task id, dataset_id, start_time and valid status are required')
  }
  if ([task.total, task.finished, task.failed].some(value => value !== undefined && (!Number.isSafeInteger(value) || value < 0))) {
    invalidResponse(endpoint, 'task progress counts must be nonnegative integers')
  }
  if ([task.dataset_version, task.dataset_version_label, task.metric_version, task.embedding_model_id, task.knowledge_base_id]
    .some(value => value !== undefined && typeof value !== 'string')) invalidResponse(endpoint, 'task snapshot identifiers must be strings')
  if (detail.params != null && (typeof detail.params !== 'object' || Array.isArray(detail.params))) {
    invalidResponse(endpoint, 'params must be a configuration object')
  }
  if (detail.metric) validateMetrics(detail.metric, endpoint)
  if (detail.results != null && !Array.isArray(detail.results)) invalidResponse(endpoint, 'results must be an array')
  for (const result of detail.results ?? []) {
    if (!result || !Number.isSafeInteger(result.qid) || typeof result.question !== 'string' ||
        typeof result.reference_answer !== 'string' || typeof result.answer !== 'string' ||
        !['pending', 'running', 'success', 'failed'].includes(result.status) || !Number.isFinite(result.duration_ms) || result.duration_ms < 0) {
      invalidResponse(endpoint, 'each result requires qid, question, reference_answer, answer, status and duration_ms')
    }
    if (result.pids != null || result.passages != null) {
      if (!Array.isArray(result.pids) || !result.pids.every(Number.isSafeInteger) ||
          !Array.isArray(result.passages) || !result.passages.every(passage => typeof passage === 'string') ||
          result.pids.length !== result.passages.length) invalidResponse(endpoint, 'reference pids and passages must match')
    }
    if (result.usage && ![result.usage.prompt_tokens, result.usage.completion_tokens, result.usage.total_tokens]
      .every(value => Number.isSafeInteger(value) && value >= 0)) invalidResponse(endpoint, 'token usage must contain nonnegative integers')
    if (result.metric) validateMetrics(result.metric, endpoint)
    if (result.steps != null && !Array.isArray(result.steps)) invalidResponse(endpoint, 'steps must be an array')
    for (const step of result.steps ?? []) {
      if (!step || typeof step.id !== 'string' || typeof step.name !== 'string' ||
          !['success', 'failed'].includes(step.status) || !Number.isFinite(step.duration_ms) ||
          typeof step.input !== 'string' || typeof step.output !== 'string') {
        invalidResponse(endpoint, 'each step requires id, name, status, duration_ms, input and output')
      }
      if (step.metric) {
        validateMetrics(step.metric, endpoint)
        if (step.metric_scope !== 'retrieval' && step.metric_scope !== 'generation') invalidResponse(endpoint, 'scored steps require metric_scope')
      }
      if (step.documents != null && !Array.isArray(step.documents)) invalidResponse(endpoint, 'documents must be an array')
      for (const document of step.documents ?? []) {
        if (!document || typeof document.id !== 'string' || typeof document.content !== 'string' ||
            typeof document.source !== 'string' || !Number.isFinite(document.score)) {
          invalidResponse(endpoint, 'each document requires id, content, source and a finite score')
        }
      }
    }
  }
}

export function validateDatasetSummary(dataset: EvaluationDatasetSummary, endpoint: string): void {
  if (!dataset || typeof dataset.id !== 'string' || typeof dataset.name !== 'string' ||
      typeof dataset.version !== 'string' || !Number.isSafeInteger(dataset.total) || dataset.total < 0 ||
      typeof dataset.description !== 'string' || typeof dataset.published_version !== 'string' ||
      typeof dataset.read_only !== 'boolean' || !Number.isSafeInteger(dataset.draft_revision)) {
    invalidResponse(endpoint, 'dataset id, name, version, total and publication metadata are required')
  }
}

export async function listEvaluationDatasets(): Promise<EvaluationDatasetSummary[]> {
  const endpoint = '/api/v1/evaluation/datasets'
  const response = await evaluationRequest(get<EvaluationResponse<EvaluationDatasetSummary[]>>(endpoint))
  if (response?.success !== true || !Array.isArray(response.data)) invalidResponse(endpoint, 'successful dataset array is required')
  response.data.forEach(dataset => validateDatasetSummary(dataset, endpoint))
  return response.data
}

export async function getEvaluationDataset(id: string, version: string): Promise<EvaluationDataset> {
  const endpoint = '/api/v1/evaluation/datasets/' + encodeURIComponent(id) + (version ? '?version=' + encodeURIComponent(version) : '')
  const response = await evaluationRequest(get<EvaluationResponse<EvaluationDataset>>(endpoint))
  if (response?.success !== true) invalidResponse(endpoint, 'success must be true')
  validateDatasetSummary(response.data, endpoint)
  if (!Array.isArray(response.data.items)) invalidResponse(endpoint, 'dataset items must be an array')
  for (const item of response.data.items) {
    if (!item || !Number.isSafeInteger(item.qid) || typeof item.question !== 'string' || typeof item.answer !== 'string' ||
        !Array.isArray(item.pids) || !item.pids.every(Number.isSafeInteger) ||
        !Array.isArray(item.passages) || !item.passages.every(passage => typeof passage === 'string') ||
        item.pids.length !== item.passages.length) {
      invalidResponse(endpoint, 'question requires qid, question, answer and matching pids/passages')
    }
  }
  if (!Array.isArray(response.data.corpus) || !response.data.corpus.every(passage => passage &&
      Number.isSafeInteger(passage.id) && typeof passage.text === 'string')) invalidResponse(endpoint, 'complete corpus is required')
  return response.data
}

export async function listEvaluationRuns(page: number, pageSize: number): Promise<EvaluationRunPage> {
  const endpoint = '/api/v1/evaluation/runs?page=' + page + '&page_size=' + pageSize
  const response = await evaluationRequest(get<EvaluationResponse<EvaluationRunPage>>(endpoint))
  if (response?.success !== true || !response.data || !Array.isArray(response.data.items) ||
      !Number.isSafeInteger(response.data.total) || !Number.isSafeInteger(response.data.page) ||
      !Number.isSafeInteger(response.data.page_size)) invalidResponse(endpoint, 'run items, total, page and page_size are required')
  response.data.items.forEach(detail => validateDetail(detail, endpoint))
  return response.data
}

export async function getEvaluationRun(id: string): Promise<EvaluationDetail> {
  const endpoint = '/api/v1/evaluation?task_id=' + encodeURIComponent(id)
  const response = await evaluationRequest(get<EvaluationResponse<EvaluationDetail>>(endpoint))
  if (response?.success !== true) invalidResponse(endpoint, 'success must be true')
  validateDetail(response.data, endpoint)
  return response.data
}

export async function startEvaluation(input: EvaluationInput): Promise<EvaluationTask> {
  const endpoint = '/api/v1/evaluation'
  const response = await evaluationRequest(post<EvaluationResponse<EvaluationDetail>>(endpoint, input))
  if (response?.success !== true) invalidResponse(endpoint, 'success must be true')
  validateDetail(response.data, endpoint)
  return response.data.task
}
