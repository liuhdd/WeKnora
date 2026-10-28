import Papa from 'papaparse'

export const DATASET_IMPORT_MAX_BYTES = 10 * 1024 * 1024
export const DATASET_MAX_CONTENT_BYTES = 8 * 1024 * 1024
export const DATASET_MAX_EXPANDED_EVIDENCE_BYTES = 8 * 1024 * 1024
export const DATASET_MAX_QUESTIONS = 1000
export const DATASET_MAX_PASSAGES = 5000
export const DATASET_MAX_TEXT_BYTES = 65536
export const DATASET_MAX_DIAGNOSTICS = 1000

export interface EvaluationDatasetQuestion {
  qid: number
  question: string
  answer: string
  pids: number[]
}

export interface EvaluationDatasetPassage {
  id: number
  text: string
}

export interface EvaluationDatasetContent {
  questions: EvaluationDatasetQuestion[]
  corpus: EvaluationDatasetPassage[]
}

export type DatasetValidationIssueCode =
  | 'invalid_json' | 'invalid_shape' | 'invalid_type' | 'invalid_id' | 'duplicate_id'
  | 'missing_reference' | 'duplicate_reference' | 'required_text' | 'text_too_large'
  | 'missing_questions' | 'missing_corpus' | 'missing_evidence' | 'duplicate_text'
  | 'too_many_questions' | 'too_many_passages' | 'file_too_large' | 'content_too_large'
  | 'expanded_evidence_too_large'
  | 'csv_parse_error' | 'csv_header' | 'csv_columns' | 'blank_row' | 'diagnostics_limit'

export interface DatasetValidationIssue {
  path: string
  code: DatasetValidationIssueCode
  message: string
}

export interface DatasetImportResult {
  content: EvaluationDatasetContent | null
  issues: DatasetValidationIssue[]
  questionCount: number
  passageCount: number
  converted: boolean
}

interface SimpleQuestion {
  question: string
  answer: string
  evidence: string[]
}

const utf8Bytes = (text: string): number => new TextEncoder().encode(text).length
const validID = (id: number): boolean => Number.isSafeInteger(id) && id >= 0
const withoutFinalTerminator = (text: string): string => text.replace(/\r\n$|\n$|\r$/, '')
const issue = (path: string, code: DatasetValidationIssueCode, message: string): DatasetValidationIssue => ({ path, code, message })

// 在遍历中限制诊断数量，达到上限就整体拒绝。 Bound diagnostics during traversal and reject the entire import at the limit.
function appendDiagnostics(existing: DatasetValidationIssue[], additions: DatasetValidationIssue[]): DatasetValidationIssue[] {
  if (existing.length > DATASET_MAX_DIAGNOSTICS) return existing
  const next = [...existing, ...additions.slice(0, DATASET_MAX_DIAGNOSTICS - existing.length)]
  return next.length < DATASET_MAX_DIAGNOSTICS ? next : [...next, issue('$', 'diagnostics_limit',
    'Too many errors; only the first ' + DATASET_MAX_DIAGNOSTICS + ' are shown. The entire import was rejected; no rows were imported.')]
}

const diagnosticsFull = (issues: DatasetValidationIssue[]): boolean => issues.length > DATASET_MAX_DIAGNOSTICS


export function cloneDatasetContent(content: EvaluationDatasetContent): EvaluationDatasetContent {
  return {
    questions: content.questions.map(question => ({ qid: question.qid, question: question.question, answer: question.answer, pids: [...question.pids] })),
    corpus: content.corpus.map(passage => ({ id: passage.id, text: passage.text })),
  }
}

function textShapeIssues(text: string, path: string): DatasetValidationIssue[] {
  if (typeof text !== 'string') return [issue(path, 'invalid_type', path + ' must be a string.')]
  if (utf8Bytes(text) > DATASET_MAX_TEXT_BYTES) {
    return [issue(path, 'text_too_large', path + ' exceeds ' + DATASET_MAX_TEXT_BYTES + ' UTF-8 bytes.')]
  }
  return []
}

// 草稿保留未完成文本，但标识和引用必须始终有效。 Drafts may contain unfinished text, while IDs and references must remain valid.
export function validateDatasetDraft(content: EvaluationDatasetContent): DatasetValidationIssue[] {
  if (!content || typeof content !== 'object' || !Array.isArray(content.questions) || !Array.isArray(content.corpus)) {
    return [issue('$', 'invalid_shape', 'Dataset must contain questions and corpus arrays; neither may be null.')]
  }
  let issues: DatasetValidationIssue[] = []
  if (content.questions.length > DATASET_MAX_QUESTIONS) issues.push(issue('questions', 'too_many_questions', 'Dataset exceeds ' + DATASET_MAX_QUESTIONS + ' questions.'))
  if (content.corpus.length > DATASET_MAX_PASSAGES) issues.push(issue('corpus', 'too_many_passages', 'Dataset exceeds ' + DATASET_MAX_PASSAGES + ' passages.'))
  if (issues.length) return issues
  const passageIDs = new Set<number>()
  for (let index = 0; index < content.corpus.length; index++) {
    const passage = content.corpus[index]
    const path = 'corpus[' + index + ']'
    if (!passage || typeof passage !== 'object') {
      issues = appendDiagnostics(issues, [issue(path, 'invalid_shape', path + ' must be an object with id and text.')])
    } else {
      if (!validID(passage.id)) issues = appendDiagnostics(issues, [issue(path + '.id', 'invalid_id', path + '.id must be a nonnegative JavaScript safe integer.')])
      else if (passageIDs.has(passage.id)) issues = appendDiagnostics(issues, [issue(path + '.id', 'duplicate_id', 'Passage ID ' + passage.id + ' is duplicated.')])
      else passageIDs.add(passage.id)
      issues = appendDiagnostics(issues, textShapeIssues(passage.text, path + '.text'))
    }
    if (diagnosticsFull(issues)) return issues
  }
  const questionIDs = new Set<number>()
  for (let index = 0; index < content.questions.length; index++) {
    const question = content.questions[index]
    const path = 'questions[' + index + ']'
    if (!question || typeof question !== 'object') {
      issues = appendDiagnostics(issues, [issue(path, 'invalid_shape', path + ' must be an object with qid, question, answer and pids.')])
      if (diagnosticsFull(issues)) return issues
      continue
    }
    if (!validID(question.qid)) issues = appendDiagnostics(issues, [issue(path + '.qid', 'invalid_id', path + '.qid must be a nonnegative JavaScript safe integer.')])
    else if (questionIDs.has(question.qid)) issues = appendDiagnostics(issues, [issue(path + '.qid', 'duplicate_id', 'Question ID ' + question.qid + ' is duplicated.')])
    else questionIDs.add(question.qid)
    issues = appendDiagnostics(issues, [...textShapeIssues(question.question, path + '.question'), ...textShapeIssues(question.answer, path + '.answer')])
    if (diagnosticsFull(issues)) return issues
    if (!Array.isArray(question.pids)) {
      issues = appendDiagnostics(issues, [issue(path + '.pids', 'invalid_type', path + '.pids must be an array and may not be null.')])
      if (diagnosticsFull(issues)) return issues
      continue
    }
    const seenPIDs = new Set<number>()
    for (let pidIndex = 0; pidIndex < question.pids.length; pidIndex++) {
      const pid = question.pids[pidIndex]
      const pidPath = path + '.pids[' + pidIndex + ']'
      if (!validID(pid)) issues = appendDiagnostics(issues, [issue(pidPath, 'invalid_id', pidPath + ' must be a nonnegative JavaScript safe integer.')])
      else if (seenPIDs.has(pid)) issues = appendDiagnostics(issues, [issue(pidPath, 'duplicate_reference', 'Question ' + question.qid + ' references passage ' + pid + ' more than once.')])
      else {
        seenPIDs.add(pid)
        if (!passageIDs.has(pid)) issues = appendDiagnostics(issues, [issue(pidPath, 'missing_reference', 'Passage ' + pid + ' referenced by question ' + question.qid + ' does not exist in corpus.')])
      }
      if (diagnosticsFull(issues)) return issues
    }
  }
  if (issues.length === 0 && utf8Bytes(JSON.stringify(cloneDatasetContent(content))) > DATASET_MAX_CONTENT_BYTES) {
    issues.push(issue('$', 'content_too_large', 'Canonical dataset content exceeds ' + DATASET_MAX_CONTENT_BYTES + ' UTF-8 bytes.'))
  }
  return issues
}

export function validateDatasetPublication(content: EvaluationDatasetContent): DatasetValidationIssue[] {
  let issues = validateDatasetDraft(content)
  if (diagnosticsFull(issues) || !content || !Array.isArray(content.questions) || !Array.isArray(content.corpus)) return issues
  if (content.questions.length > DATASET_MAX_QUESTIONS || content.corpus.length > DATASET_MAX_PASSAGES) return issues
  if (content.questions.length === 0) issues = appendDiagnostics(issues, [issue('questions', 'missing_questions', 'At least one question is required for import or publication.')])
  if (content.corpus.length === 0) issues = appendDiagnostics(issues, [issue('corpus', 'missing_corpus', 'At least one evidence passage is required for import or publication.')])
  if (diagnosticsFull(issues)) return issues
  const textIDs = new Map<string, number>()
  for (let index = 0; index < content.corpus.length; index++) {
    const passage = content.corpus[index]
    if (!passage || typeof passage.text !== 'string') continue
    if (!passage.text.trim()) issues = appendDiagnostics(issues, [issue('corpus[' + index + '].text', 'required_text', 'Passage ' + passage.id + ' requires nonempty text.')])
    else if (textIDs.has(passage.text)) {
      issues = appendDiagnostics(issues, [issue('corpus[' + index + '].text', 'duplicate_text', 'Passage ' + passage.id + ' duplicates the exact text of passage ' + textIDs.get(passage.text) + '; reference the existing passage ID instead.')])
    } else textIDs.set(passage.text, passage.id)
    if (diagnosticsFull(issues)) return issues
  }
  for (let index = 0; index < content.questions.length; index++) {
    const question = content.questions[index]
    if (!question) continue
    if (typeof question.question === 'string' && !question.question.trim()) issues = appendDiagnostics(issues, [issue('questions[' + index + '].question', 'required_text', 'Question ' + question.qid + ' requires nonempty question text.')])
    if (typeof question.answer === 'string' && !question.answer.trim()) issues = appendDiagnostics(issues, [issue('questions[' + index + '].answer', 'required_text', 'Question ' + question.qid + ' requires a nonempty reference answer.')])
    if (Array.isArray(question.pids) && question.pids.length === 0) issues = appendDiagnostics(issues, [issue('questions[' + index + '].pids', 'missing_evidence', 'Question ' + question.qid + ' requires at least one evidence passage ID.')])
    if (diagnosticsFull(issues)) return issues
  }
  return appendDiagnostics(issues, expandedEvidenceIssues(content))
}

// 每次引用都计入展开预算，不通过删除引用缩减数据。 Count every evidence reference without dropping references to fit the budget.
function expandedEvidenceIssues(content: EvaluationDatasetContent): DatasetValidationIssue[] {
  const passageBytes = new Map<number, number>()
  for (const passage of content.corpus) {
    if (passage && validID(passage.id) && typeof passage.text === 'string' && !passageBytes.has(passage.id)) {
      passageBytes.set(passage.id, utf8Bytes(passage.text))
    }
  }
  let expandedBytes = 0
  for (let questionIndex = 0; questionIndex < content.questions.length; questionIndex++) {
    const question = content.questions[questionIndex]
    if (!question || !Array.isArray(question.pids)) continue
    for (let pidIndex = 0; pidIndex < question.pids.length; pidIndex++) {
      const bytes = passageBytes.get(question.pids[pidIndex])
      if (bytes === undefined) continue
      expandedBytes += bytes
      if (expandedBytes > DATASET_MAX_EXPANDED_EVIDENCE_BYTES) {
        return [issue('questions[' + questionIndex + '].pids[' + pidIndex + ']', 'expanded_evidence_too_large',
          'Evidence expanded across all question references exceeds ' + DATASET_MAX_EXPANDED_EVIDENCE_BYTES +
          ' UTF-8 bytes. Every reference, including shared evidence, counts; no references were dropped.')]
      }
    }
  }
  return []
}

function fileIssues(text: string): DatasetValidationIssue[] {
  return utf8Bytes(text) > DATASET_IMPORT_MAX_BYTES
    ? [issue('$', 'file_too_large', 'Import file exceeds ' + DATASET_IMPORT_MAX_BYTES + ' UTF-8 bytes.')]
    : []
}

export function parseDatasetJSON(text: string): DatasetImportResult {
  const issues = fileIssues(text)
  if (issues.length) return { content: null, issues, questionCount: 0, passageCount: 0, converted: false }
  let content: EvaluationDatasetContent
  try {
    content = JSON.parse(text)
  } catch (error) {
    if (!(error instanceof SyntaxError)) throw error
    return { content: null, issues: [issue('$', 'invalid_json', error.message)], questionCount: 0, passageCount: 0, converted: false }
  }
  const validation = validateDatasetPublication(content)
  return { content: validation.length ? null : cloneDatasetContent(content), issues: validation,
    questionCount: Array.isArray(content?.questions) ? content.questions.length : 0,
    passageCount: Array.isArray(content?.corpus) ? content.corpus.length : 0, converted: false }
}

function simpleQuestionIssues(question: SimpleQuestion, path: string): DatasetValidationIssue[] {
  if (!question || typeof question !== 'object') return [issue(path, 'invalid_shape', path + ' must contain question, answer and evidence.')]
  let issues = [...textShapeIssues(question.question, path + '.question'), ...textShapeIssues(question.answer, path + '.answer')]
  if (typeof question.question === 'string' && !question.question.trim()) issues = appendDiagnostics(issues, [issue(path + '.question', 'required_text', path + '.question must not be blank.')])
  if (typeof question.answer === 'string' && !question.answer.trim()) issues = appendDiagnostics(issues, [issue(path + '.answer', 'required_text', path + '.answer must not be blank.')])
  if (!Array.isArray(question.evidence)) return appendDiagnostics(issues, [issue(path + '.evidence', 'invalid_type', path + '.evidence must be an array of nonempty strings.')])
  if (!question.evidence.length) issues = appendDiagnostics(issues, [issue(path + '.evidence', 'missing_evidence', path + '.evidence requires at least one passage.')])
  for (let index = 0; index < question.evidence.length; index++) {
    const text = question.evidence[index]
    const evidencePath = path + '.evidence[' + index + ']'
    issues = appendDiagnostics(issues, textShapeIssues(text, evidencePath))
    if (typeof text === 'string' && !text.trim()) issues = appendDiagnostics(issues, [issue(evidencePath, 'required_text', evidencePath + ' must not be blank.')])
    if (diagnosticsFull(issues)) return issues
  }
  return issues
}

// 简易导入只合并完全相同的证据，不合并或删除题目。 Simple imports merge exact evidence text without merging or dropping questions.
function simpleQuestionsContent(rows: SimpleQuestion[]): EvaluationDatasetContent {
  const corpus: EvaluationDatasetPassage[] = []
  const textIDs = new Map<string, number>()
  const questions = rows.map((row, qid) => {
    const pids = row.evidence.map(text => {
      const existing = textIDs.get(text)
      if (existing !== undefined) return existing
      const id = corpus.length
      textIDs.set(text, id)
      corpus.push({ id, text })
      return id
    })
    return { qid, question: row.question, answer: row.answer, pids: [...new Set(pids)] }
  })
  return { questions, corpus }
}

export function parseDatasetJSONL(text: string): DatasetImportResult {
  let issues = fileIssues(text)
  if (issues.length) return { content: null, issues, questionCount: 0, passageCount: 0, converted: true }
  const lines = text.length ? withoutFinalTerminator(text).split(/\r\n|\r|\n/, DATASET_MAX_QUESTIONS + 1) : []
  if (lines.length > DATASET_MAX_QUESTIONS) {
    return { content: null, issues: [issue('lines', 'too_many_questions', 'Import exceeds ' + DATASET_MAX_QUESTIONS + ' question records; no rows were imported.')],
      questionCount: lines.length, passageCount: 0, converted: true }
  }
  const rows: SimpleQuestion[] = []
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index]
    const path = 'line ' + (index + 1)
    if (!line.trim()) issues = appendDiagnostics(issues, [issue(path, 'blank_row', path + ' is blank; remove the empty record.')])
    else {
      let question: SimpleQuestion
      try {
        question = JSON.parse(line)
      } catch (error) {
        if (!(error instanceof SyntaxError)) throw error
        issues = appendDiagnostics(issues, [issue(path, 'invalid_json', error.message)])
        if (diagnosticsFull(issues)) return { content: null, issues, questionCount: lines.length, passageCount: 0, converted: true }
        continue
      }
      const rowIssues = simpleQuestionIssues(question, path)
      issues = appendDiagnostics(issues, rowIssues)
      if (!rowIssues.length) rows.push(question)
    }
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: lines.length, passageCount: 0, converted: true }
  }
  const content = simpleQuestionsContent(rows)
  issues = appendDiagnostics(issues, validateDatasetPublication(content))
  return { content: issues.length ? null : content, issues, questionCount: lines.length, passageCount: content.corpus.length, converted: true }
}

function csvQuoteIssues(text: string): DatasetValidationIssue[] {
  let state: 'start' | 'plain' | 'quoted' | 'closed' = 'start'
  let line = 1
  for (let index = 0; index < text.length; index++) {
    const character = text[index]
    if (index === 0 && character === '\ufeff') continue
    if (state === 'quoted') {
      if (character === '"' && text[index + 1] === '"') index++
      else if (character === '"') state = 'closed'
    } else if (character === '"') {
      if (state !== 'start') return [issue('line ' + line, 'csv_parse_error', 'Unexpected quote inside an unquoted CSV field at line ' + line + '.')]
      state = 'quoted'
    } else if (character === ',') state = 'start'
    else if (character === '\n' || character === '\r') state = 'start'
    else if (state === 'closed') return [issue('line ' + line, 'csv_parse_error', 'Unexpected text after a closing CSV quote at line ' + line + '.')]
    else state = 'plain'
    if (character === '\n' || (character === '\r' && text[index + 1] !== '\n')) line++
  }
  return state === 'quoted' ? [issue('line ' + line, 'csv_parse_error', 'CSV quoted field is not closed before the end of the file.')] : []
}

export function parseDatasetCSV(text: string): DatasetImportResult {
  let issues = fileIssues(text)
  if (issues.length) return { content: null, issues, questionCount: 0, passageCount: 0, converted: true }
  const input = withoutFinalTerminator(text)
  issues = appendDiagnostics(issues, csvQuoteIssues(input))
  if (issues.length) return { content: null, issues, questionCount: 0, passageCount: 0, converted: true }
  const parsed = Papa.parse<string[]>(input, { delimiter: ',', header: false, dynamicTyping: false,
    skipEmptyLines: false, comments: false, quoteChar: '"', escapeChar: '"', fastMode: false, download: false, worker: false,
    preview: DATASET_MAX_QUESTIONS + 2 })
  for (const error of parsed.errors) {
    issues = appendDiagnostics(issues, [issue('record ' + ((error.row ?? 0) + 1), 'csv_parse_error', error.code + ': ' + error.message)])
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: parsed.data.length - 1, passageCount: 0, converted: true }
  }
  if (parsed.meta.truncated || parsed.data.length - 1 > DATASET_MAX_QUESTIONS) {
    issues = appendDiagnostics(issues, [issue('records', 'too_many_questions', 'Import exceeds ' + DATASET_MAX_QUESTIONS + ' question records; no rows were imported.')])
    return { content: null, issues, questionCount: parsed.data.length - 1, passageCount: 0, converted: true }
  }
  const headers = parsed.data[0] ?? []
  const seenHeaders = new Set<string>()
  for (let index = 0; index < headers.length; index++) {
    const header = headers[index]
    if (!header) issues = appendDiagnostics(issues, [issue('header[' + index + ']', 'csv_header', 'CSV column ' + (index + 1) + ' requires a header name.')])
    else if (seenHeaders.has(header)) issues = appendDiagnostics(issues, [issue('header[' + index + ']', 'csv_header', 'Duplicate CSV header: ' + header + '.')])
    else seenHeaders.add(header)
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: parsed.data.length - 1, passageCount: 0, converted: true }
  }
  for (const header of ['question', 'answer', 'evidence']) {
    if (!seenHeaders.has(header)) issues = appendDiagnostics(issues, [issue('header', 'csv_header', 'Required CSV header is missing: ' + header + '.')])
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: parsed.data.length - 1, passageCount: 0, converted: true }
  }
  const evidenceNumbers: number[] = []
  for (const header of headers) {
    if (!/^evidence_/.test(header)) continue
    const suffix = header.slice('evidence_'.length)
    if (!/^[1-9]\d*$/.test(suffix) || !Number.isSafeInteger(Number(suffix)) || Number(suffix) < 2) {
      issues = appendDiagnostics(issues, [issue('header', 'csv_header', 'Invalid evidence column ' + header + '; use evidence_2, evidence_3, and so on.')])
    } else evidenceNumbers.push(Number(suffix))
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: parsed.data.length - 1, passageCount: 0, converted: true }
  }
  evidenceNumbers.sort((left, right) => left - right)
  for (let index = 0; index < evidenceNumbers.length; index++) {
    if (evidenceNumbers[index] !== index + 2) issues = appendDiagnostics(issues, [issue('header', 'csv_header', 'Optional evidence columns must be sequential, starting with evidence_2.')])
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: parsed.data.length - 1, passageCount: 0, converted: true }
  }
  const questionIndex = headers.indexOf('question')
  const answerIndex = headers.indexOf('answer')
  const evidenceIndexes = [headers.indexOf('evidence'), ...evidenceNumbers.map(number => headers.indexOf('evidence_' + number))]
  const dataRows = parsed.data.slice(1)
  const rows: SimpleQuestion[] = []
  for (let index = 0; index < dataRows.length; index++) {
    const row = dataRows[index]
    const path = 'record ' + (index + 2)
    if (row.every(cell => !cell.trim())) issues = appendDiagnostics(issues, [issue(path, 'blank_row', path + ' is blank; remove the empty record.')])
    else if (row.length !== headers.length) {
      issues = appendDiagnostics(issues, [issue(path, 'csv_columns', path + ' has ' + row.length + ' fields; expected ' + headers.length + '.')])
    } else if (questionIndex >= 0 && answerIndex >= 0 && !evidenceIndexes.some(column => column < 0)) {
      const question: SimpleQuestion = { question: row[questionIndex], answer: row[answerIndex],
        evidence: evidenceIndexes.filter((column, evidenceIndex) => evidenceIndex === 0 || row[column].trim()).map(column => row[column]) }
      const rowIssues = simpleQuestionIssues(question, path)
      issues = appendDiagnostics(issues, rowIssues)
      if (!rowIssues.length) rows.push(question)
    }
    if (diagnosticsFull(issues)) return { content: null, issues, questionCount: dataRows.length, passageCount: 0, converted: true }
  }
  const content = simpleQuestionsContent(rows)
  issues = appendDiagnostics(issues, validateDatasetPublication(content))
  return { content: issues.length ? null : content, issues, questionCount: dataRows.length, passageCount: content.corpus.length, converted: true }
}

export function datasetJSONTemplate(): string {
  return JSON.stringify({ questions: [{ qid: 0, question: 'How long are backups retained?', answer: 'Backups are retained for seven days.', pids: [0] }],
    corpus: [{ id: 0, text: 'Backups are retained for seven days.' }, { id: 1, text: 'Unreferenced passages remain in the corpus for retrieval evaluation.' }] }, null, 2)
}

export function datasetCSVTemplate(): string {
  return Papa.unparse({ fields: ['question', 'answer', 'evidence', 'evidence_2'], data: [
    ['How long are backups retained?', 'Backups are retained for seven days.', 'Backups are retained for seven days.', ''],
  ] }, { quotes: true, quoteChar: '"', escapeChar: '"', delimiter: ',', header: true, newline: '\r\n', skipEmptyLines: false, escapeFormulae: false })
}
