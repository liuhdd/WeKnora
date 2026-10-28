import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DATASET_IMPORT_MAX_BYTES, DATASET_MAX_CONTENT_BYTES, DATASET_MAX_DIAGNOSTICS,
  DATASET_MAX_EXPANDED_EVIDENCE_BYTES, DATASET_MAX_QUESTIONS, DATASET_MAX_TEXT_BYTES,
  cloneDatasetContent, datasetCSVTemplate, datasetJSONTemplate, parseDatasetCSV, parseDatasetJSON,
  parseDatasetJSONL, validateDatasetDraft, validateDatasetPublication, type EvaluationDatasetContent,
} from './evaluationDatasetImport.ts'

test('canonical JSON keeps IDs, unreferenced corpus and exact text through a round trip', () => {
  const content: EvaluationDatasetContent = {
    questions: [{ qid: 41, question: 'Retention?', answer: 'Seven days.', pids: [12] }],
    corpus: [{ id: 12, text: 'Seven days.\nKeep this newline.' }, { id: 99, text: 'Unrelated retrieval candidate.' }],
  }
  const result = parseDatasetJSON(JSON.stringify(content))
  assert.deepEqual(result.issues, [])
  assert.deepEqual(result.content, content)
  assert.equal(result.converted, false)
  const clone = cloneDatasetContent(content)
  clone.questions[0].pids.push(99)
  clone.corpus[0].text = 'Changed locally.'
  assert.deepEqual(content.questions[0].pids, [12])
  assert.equal(content.corpus[0].text, 'Seven days.\nKeep this newline.')
})

test('canonical JSON rejects duplicate IDs, repeated references and duplicate passage text without normalizing them', () => {
  const result = parseDatasetJSON(JSON.stringify({
    questions: [{ qid: 1, question: 'Q', answer: 'A', pids: [2, 2, 77] }, { qid: 1, question: 'Q2', answer: 'A2', pids: [2] }],
    corpus: [{ id: 2, text: 'Same' }, { id: 3, text: 'Same' }, { id: 2, text: 'Another' }],
  }))
  assert.equal(result.content, null)
  const codes = new Set(result.issues.map(issue => issue.code))
  assert.ok(codes.has('duplicate_id'))
  assert.ok(codes.has('duplicate_reference'))
  assert.ok(codes.has('missing_reference'))
  assert.ok(codes.has('duplicate_text'))
  assert.equal(result.questionCount, 2)
  assert.equal(result.passageCount, 3)
})

test('unfinished drafts are saveable but required fields, safe IDs and references remain structural invariants', () => {
  const draft: EvaluationDatasetContent = { questions: [{ qid: 0, question: '', answer: '', pids: [] }], corpus: [{ id: 0, text: '' }] }
  assert.deepEqual(validateDatasetDraft(draft), [])
  assert.ok(validateDatasetPublication(draft).some(issue => issue.code === 'missing_evidence'))
  assert.ok(validateDatasetPublication(draft).some(issue => issue.code === 'required_text'))
  assert.deepEqual(validateDatasetDraft({ questions: [], corpus: [] }), [])
  assert.equal(parseDatasetJSON('{"questions":[{"qid":0,"question":"Q","answer":"A","pids":null}],"corpus":[]}').content, null)
  assert.equal(parseDatasetJSON('{"questions":[],"corpus":null}').content, null)
  assert.ok(validateDatasetDraft({ questions: [{ qid: Number.MAX_SAFE_INTEGER + 1, question: '', answer: '', pids: [2] }], corpus: [] })
    .some(issue => issue.code === 'invalid_id'))
})

test('JSONL generates stable IDs and shares exact evidence while preserving every question record', () => {
  const result = parseDatasetJSONL([
    JSON.stringify({ question: 'Q1', answer: 'A1', evidence: ['Shared', 'Shared'] }),
    JSON.stringify({ question: 'Q1', answer: 'A1', evidence: ['Shared', 'Other'] }),
    '',
  ].join('\r\n'))
  assert.deepEqual(result.issues, [])
  assert.equal(result.converted, true)
  assert.equal(result.questionCount, 2)
  assert.deepEqual(result.content?.questions.map(question => question.qid), [0, 1])
  assert.deepEqual(result.content?.questions.map(question => question.pids), [[0], [0, 1]])
  assert.deepEqual(result.content?.corpus, [{ id: 0, text: 'Shared' }, { id: 1, text: 'Other' }])
})

test('JSONL errors are atomic and report malformed, blank and incomplete records instead of dropping rows', () => {
  assert.equal(parseDatasetJSONL('').questionCount, 0)
  const result = parseDatasetJSONL('{"question":"Q","answer":"A","evidence":["E"]}\n \n{bad}\n{"question":"Q2","answer":"","evidence":[]}\n')
  assert.equal(result.content, null)
  assert.equal(result.questionCount, 4)
  assert.ok(result.issues.some(issue => issue.path === 'line 2' && issue.code === 'blank_row'))
  assert.ok(result.issues.some(issue => issue.path === 'line 3' && issue.code === 'invalid_json'))
  assert.ok(result.issues.some(issue => issue.path === 'line 4.answer'))
})

test('CSV uses the real parser for quoted multiline evidence, escapes and optional shared passages', () => {
  const result = parseDatasetCSV('question,answer,evidence,evidence_2\r\n"What is \"\"backup\"\"?",A,"Line one\nLine two",Shared\r\nQ2,A2,Shared,\r\n')
  assert.deepEqual(result.issues, [])
  assert.equal(result.questionCount, 2)
  assert.equal(result.content?.questions[0].question, 'What is "backup"?')
  assert.deepEqual(result.content?.corpus, [{ id: 0, text: 'Line one\nLine two' }, { id: 1, text: 'Shared' }])
  assert.deepEqual(result.content?.questions[1].pids, [1])
})

test('CSV rejects duplicate headers, inconsistent fields, blank interior rows and malformed quotes', () => {
  const cases = [
    'question,answer,evidence,evidence\nQ,A,E,E2',
    'question,answer,evidence\nQ,A,E\nQ2,A2',
    'question,answer,evidence\nQ,A,E\n\nQ2,A2,E2',
    'question,answer,evidence\nQ,A,"Unclosed',
    'question,answer,evidence\nQ,A,bare"quote',
    'question,answer,evidence,evidence_3\nQ,A,E,E3',
  ]
  for (const source of cases) {
    const result = parseDatasetCSV(source)
    assert.equal(result.content, null, source)
    assert.ok(result.issues.length > 0, source)
  }
  assert.equal(parseDatasetCSV('question,answer,evidence\nQ,A,E\n\n').questionCount, 2)
})

test('UTF-8 field sizes, canonical bytes and question counts enforce backend bounds', () => {
  const oversizedText = '证'.repeat(Math.floor(DATASET_MAX_TEXT_BYTES / 3) + 1)
  assert.ok(validateDatasetDraft({ questions: [], corpus: [{ id: 0, text: oversizedText }] }).some(issue => issue.code === 'text_too_large'))
  const tooMany = Array.from({ length: DATASET_MAX_QUESTIONS + 1 }, (_, qid) => ({ qid, question: '', answer: '', pids: [] }))
  assert.ok(validateDatasetDraft({ questions: tooMany, corpus: [] }).some(issue => issue.code === 'too_many_questions'))
  assert.equal(parseDatasetCSV('question,answer,evidence\n' + Array.from({ length: DATASET_MAX_QUESTIONS }, () => 'Q,A,E').join('\n')).issues.length, 0)
  assert.ok(parseDatasetCSV('question,answer,evidence\n' + Array.from({ length: DATASET_MAX_QUESTIONS + 1 }, () => 'Q,A,E').join('\n'))
    .issues.some(issue => issue.code === 'too_many_questions'))
  const largeContent = { questions: [], corpus: Array.from({ length: Math.ceil(DATASET_MAX_CONTENT_BYTES / 60000) }, (_, id) => ({ id, text: String(id) + 'x'.repeat(60000) })) }
  assert.ok(validateDatasetDraft(largeContent).some(issue => issue.code === 'content_too_large'))
})

test('downloadable templates import without losing their intended corpus', () => {
  assert.deepEqual(parseDatasetJSON(datasetJSONTemplate()).issues, [])
  assert.equal(parseDatasetJSON(datasetJSONTemplate()).content?.corpus.length, 2)
  assert.deepEqual(parseDatasetCSV(datasetCSVTemplate()).issues, [])
  assert.deepEqual(parseDatasetCSV('\ufeff' + datasetCSVTemplate()).issues, [])
})

test('publication counts every shared evidence reference against the expanded UTF-8 budget without changing drafts', () => {
  const content: EvaluationDatasetContent = {
    questions: Array.from({ length: DATASET_MAX_QUESTIONS }, (_, qid) => ({ qid, question: 'Q', answer: 'A', pids: [0, 1] })),
    corpus: [{ id: 0, text: 'a'.repeat(DATASET_MAX_TEXT_BYTES) }, { id: 1, text: 'b'.repeat(DATASET_MAX_TEXT_BYTES) }],
  }
  const serialized = JSON.stringify(content)
  assert.ok(new TextEncoder().encode(serialized).length < DATASET_MAX_CONTENT_BYTES)
  assert.deepEqual(validateDatasetDraft(content), [])
  const result = parseDatasetJSON(serialized)
  assert.equal(result.content, null)
  assert.ok(result.issues.some(issue => issue.code === 'expanded_evidence_too_large' && issue.path.includes('.pids[')))
  assert.equal(JSON.stringify(content), serialized)

  const exact: EvaluationDatasetContent = {
    questions: Array.from({ length: DATASET_MAX_EXPANDED_EVIDENCE_BYTES / DATASET_MAX_TEXT_BYTES }, (_, qid) => ({ qid, question: 'Q', answer: 'A', pids: [0] })),
    corpus: [{ id: 0, text: '证'.repeat(Math.floor(DATASET_MAX_TEXT_BYTES / 3)) + 'x' }],
  }
  assert.deepEqual(validateDatasetPublication(exact), [])
  assert.deepEqual(validateDatasetPublication({ questions: exact.questions.slice(0, 2), corpus: exact.corpus }), [])
  const invalid: EvaluationDatasetContent = JSON.parse('{"questions":[{"qid":0,"question":"Q","answer":"A","pids":[999,999]},{"qid":1,"question":"Q2","answer":"A2","pids":null}],"corpus":[{"id":0,"text":"E"},{"id":0,"text":"E2"}]}')
  assert.ok(validateDatasetPublication(invalid).length > 0)
})

test('diagnostics stop at their budget across canonical references, publication and simple format records', () => {
  const repeated: EvaluationDatasetContent = {
    questions: Array.from({ length: DATASET_MAX_QUESTIONS }, (_, qid) => ({ qid, question: 'Q', answer: 'A', pids: Array.from({ length: 1200 }, () => 0) })),
    corpus: [{ id: 0, text: 'Shared evidence' }],
  }
  const before = JSON.stringify(repeated)
  assert.ok(new TextEncoder().encode(before).length < DATASET_IMPORT_MAX_BYTES)
  const canonical = parseDatasetJSON(before)
  assert.equal(canonical.content, null)
  assert.equal(canonical.issues.length, DATASET_MAX_DIAGNOSTICS + 1)
  assert.equal(canonical.issues.at(-1)?.code, 'diagnostics_limit')
  assert.equal(JSON.stringify(repeated), before)

  const unfinished: EvaluationDatasetContent = { questions: Array.from({ length: DATASET_MAX_QUESTIONS }, (_, qid) => ({ qid, question: '', answer: '', pids: [] })), corpus: [] }
  assert.deepEqual(validateDatasetDraft(unfinished), [])
  const publication = validateDatasetPublication(unfinished)
  assert.equal(publication.length, DATASET_MAX_DIAGNOSTICS + 1)
  assert.equal(publication.at(-1)?.code, 'diagnostics_limit')
  const jsonl = parseDatasetJSONL(JSON.stringify({ question: 'Q', answer: 'A', evidence: Array.from({ length: 2000 }, () => '') }))
  assert.equal(jsonl.content, null)
  assert.equal(jsonl.issues.length, DATASET_MAX_DIAGNOSTICS + 1)
  const csv = parseDatasetCSV('question,answer,evidence,' + ','.repeat(2000) + '\nQ,A,E')
  assert.equal(csv.content, null)
  assert.equal(csv.issues.length, DATASET_MAX_DIAGNOSTICS + 1)
})

test('newline-only JSONL and heavily malformed CSV reject before creating unbounded row or error arrays', () => {
  const jsonl = parseDatasetJSONL('\n'.repeat(DATASET_IMPORT_MAX_BYTES))
  assert.equal(jsonl.content, null)
  assert.ok(jsonl.questionCount <= DATASET_MAX_QUESTIONS + 1)
  assert.equal(jsonl.issues[0].code, 'too_many_questions')
  assert.ok(jsonl.issues.length <= DATASET_MAX_DIAGNOSTICS + 1)
  const csv = parseDatasetCSV('question,answer,evidence\nQ,A,"a"' + '"invalid'.repeat(100000))
  assert.equal(csv.content, null)
  assert.equal(csv.issues.length, 1)
  assert.equal(csv.issues[0].code, 'csv_parse_error')
})
