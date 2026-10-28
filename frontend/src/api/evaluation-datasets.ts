import { get, post, put } from '@/utils/request'
import { evaluationRequest, type EvaluationDataset } from './evaluation'
import { validateDatasetDraft, type DatasetValidationIssue, type EvaluationDatasetContent } from '@/views/evaluation/evaluationDatasetImport'

export type { EvaluationDatasetContent } from '@/views/evaluation/evaluationDatasetImport'

export interface EvaluationDatasetDraft {
  id: string
  name: string
  description: string
  revision: number
  content: EvaluationDatasetContent
  updated_at: string
}

export interface EvaluationDatasetVersion {
  version: string
  content_hash: string
  total: number
  created_at?: string
  name: string
  description: string
}

export interface EvaluationDatasetInput {
  name: string
  description: string
  content: EvaluationDatasetContent
}

export interface EvaluationDatasetDraftInput extends EvaluationDatasetInput {
  revision: number
}

interface DatasetResponse<T> { success: boolean; data: T }
const base = '/api/v1/evaluation/datasets'

export function evaluationDatasetMetadataIssues(name: string, description: string): DatasetValidationIssue[] {
  const issues: DatasetValidationIssue[] = []
  if (!name.trim()) issues.push({ path: 'name', code: 'required_text', message: 'Dataset name is required.' })
  if (Array.from(name).length > 200) issues.push({ path: 'name', code: 'text_too_large', message: 'Dataset name must not exceed 200 characters.' })
  if (Array.from(description).length > 2000) issues.push({ path: 'description', code: 'text_too_large', message: 'Dataset description must not exceed 2000 characters.' })
  return issues
}

function validateDraft(draft: EvaluationDatasetDraft, endpoint: string): void {
  if (!draft || typeof draft.id !== 'string' || typeof draft.name !== 'string' || typeof draft.description !== 'string' ||
      !Number.isSafeInteger(draft.revision) || draft.revision < 1 || typeof draft.updated_at !== 'string') {
    throw new TypeError('Invalid dataset draft response from ' + endpoint + ': metadata and revision are required')
  }
  const issues = validateDatasetDraft(draft.content)
  if (issues.length) throw new TypeError('Invalid dataset draft response from ' + endpoint + ': ' + issues[0].path + ' ' + issues[0].message)
}

function validateVersion(version: EvaluationDatasetVersion, endpoint: string): void {
  if (!version || typeof version.version !== 'string' || typeof version.content_hash !== 'string' ||
      !Number.isSafeInteger(version.total) || typeof version.name !== 'string' || typeof version.description !== 'string') {
    throw new TypeError('Invalid dataset version response from ' + endpoint + ': version, content hash, total and metadata are required')
  }
}

export async function createEvaluationDataset(input: EvaluationDatasetInput): Promise<EvaluationDatasetDraft> {
  const response = await evaluationRequest(post<DatasetResponse<EvaluationDatasetDraft>>(base, input))
  if (response?.success !== true) throw new TypeError('Create dataset response must report success')
  validateDraft(response.data, base)
  return response.data
}

export async function getEvaluationDatasetDraft(id: string): Promise<EvaluationDatasetDraft> {
  const endpoint = base + '/' + encodeURIComponent(id) + '/draft'
  const response = await evaluationRequest(get<DatasetResponse<EvaluationDatasetDraft>>(endpoint))
  if (response?.success !== true) throw new TypeError('Get dataset draft response must report success')
  validateDraft(response.data, endpoint)
  return response.data
}

export async function saveEvaluationDatasetDraft(id: string, input: EvaluationDatasetDraftInput): Promise<EvaluationDatasetDraft> {
  const endpoint = base + '/' + encodeURIComponent(id) + '/draft'
  const response = await evaluationRequest(put<DatasetResponse<EvaluationDatasetDraft>>(endpoint, input))
  if (response?.success !== true) throw new TypeError('Save dataset draft response must report success')
  validateDraft(response.data, endpoint)
  return response.data
}

export async function publishEvaluationDataset(id: string, revision: number): Promise<EvaluationDatasetVersion> {
  const endpoint = base + '/' + encodeURIComponent(id) + '/publish'
  const response = await evaluationRequest(post<DatasetResponse<EvaluationDatasetVersion>>(endpoint, { revision }))
  if (response?.success !== true) throw new TypeError('Publish dataset response must report success')
  validateVersion(response.data, endpoint)
  return response.data
}

export async function listEvaluationDatasetVersions(id: string): Promise<EvaluationDatasetVersion[]> {
  const endpoint = base + '/' + encodeURIComponent(id) + '/versions'
  const response = await evaluationRequest(get<DatasetResponse<EvaluationDatasetVersion[]>>(endpoint))
  if (response?.success !== true || !Array.isArray(response.data)) throw new TypeError('Dataset versions response must be an array')
  response.data.forEach(version => validateVersion(version, endpoint))
  return response.data
}

// 发布预览包含完整语料，复制与恢复不会遗漏干扰段落。 Preserve the full corpus when copying or restoring a published snapshot.
export function evaluationContentFromVersion(dataset: EvaluationDataset): EvaluationDatasetContent {
  return {
    questions: dataset.items.map(item => ({ qid: item.qid, question: item.question, answer: item.answer, pids: [...item.pids] })),
    corpus: dataset.corpus.map(passage => ({ id: passage.id, text: passage.text })),
  }
}
