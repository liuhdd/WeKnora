<template>
  <t-dialog :visible="visible" :header="t('evaluation.importDataset')" :footer="false" width="800px" top="6vh"
    dialog-class-name="evaluation-create-dialog" destroy-on-close @update:visible="(value: boolean) => !value && requestClose()">
    <form class="evaluation-create-form eval-import-form" data-testid="dataset-import-form" @submit.prevent="saveImport">
      <p>{{ t('evaluation.importFormats') }}</p>
      <p>{{ t('evaluation.datasetLimits') }}</p>
      <div class="eval-import-templates"><t-button size="small" theme="default" variant="outline" data-testid="dataset-json-template" @click="downloadJSONTemplate">{{ t('evaluation.jsonTemplate') }}</t-button>
        <t-button size="small" theme="default" variant="outline" data-testid="dataset-csv-template" @click="downloadCSVTemplate">{{ t('evaluation.csvTemplate') }}</t-button></div>
      <label>{{ t('evaluation.importFile') }}<input type="file" accept=".json,.jsonl,.csv" data-testid="dataset-import-file" :disabled="saving" @change="readFile" /></label>
      <t-loading v-if="reading" size="small" />
      <t-alert v-if="error" theme="error" :message="error" />
      <EvaluationDatasetIssues v-if="result" :issues="result.issues" />
      <template v-if="result?.content">
        <t-alert v-if="result.converted" theme="info" :message="t('evaluation.importConversionHint')" />
        <p>{{ fileName }} · {{ t('evaluation.questionCount') }} {{ result.questionCount }} · {{ t('evaluation.datasetCorpus') }} {{ result.passageCount }}</p>
        <label>{{ t('evaluation.datasetName') }}<input v-model="name" type="text" data-testid="dataset-import-name" :disabled="saving" required /></label>
        <label>{{ t('evaluation.datasetDescription') }}<textarea v-model="description" rows="2" data-testid="dataset-import-description" :disabled="saving" /></label>
        <EvaluationDatasetIssues :issues="metadataIssues" />
        <div class="eval-import-preview" data-testid="dataset-import-preview"><table><thead><tr><th>QID</th><th>{{ t('evaluation.question') }}</th><th>{{ t('evaluation.referenceAnswer') }}</th><th>{{ t('evaluation.evidence') }}</th></tr></thead>
          <tbody><tr v-for="question in visibleQuestions" :key="question.qid"><td>{{ question.qid }}</td><td>{{ question.question }}</td><td>{{ question.answer }}</td>
            <td><details :open="expandedQuestionId === question.qid" @toggle="togglePreview(question.qid, $event)"><summary>{{ question.pids.length }}</summary>
              <template v-if="expandedQuestionId === question.qid"><div v-for="pid in question.pids" :key="pid"><b>PID {{ pid }}</b><p>{{ passageText(pid) }}</p></div></template>
            </details></td></tr></tbody></table></div>
        <div class="eval-import-pagination"><t-button size="small" variant="outline" theme="default" data-testid="dataset-import-previous" :disabled="previewPage === 0" @click="previewPage -= 1">‹</t-button>
          <span>{{ previewPage + 1 }} / {{ previewPages }}</span><t-button size="small" variant="outline" theme="default" data-testid="dataset-import-next" :disabled="previewPage + 1 >= previewPages" @click="previewPage += 1">›</t-button></div>
      </template>
      <div class="eval-actions"><t-button theme="default" variant="outline" data-testid="dataset-import-cancel" :disabled="saving" @click="requestClose">{{ t('common.cancel') }}</t-button>
        <t-button type="submit" data-testid="dataset-import-save" :loading="saving" :disabled="!result?.content || reading || metadataIssues.length > 0">{{ t('evaluation.importAsDraft') }}</t-button></div>
    </form>
  </t-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { createEvaluationDataset, evaluationDatasetMetadataIssues } from '@/api/evaluation-datasets'
import { DATASET_IMPORT_MAX_BYTES, datasetCSVTemplate, datasetJSONTemplate, parseDatasetCSV, parseDatasetJSON,
  parseDatasetJSONL, type DatasetImportResult } from './evaluationDatasetImport'
import EvaluationDatasetIssues from './EvaluationDatasetIssues.vue'

const props = defineProps<{ visible: boolean; confirmDiscard: (body: string) => Promise<boolean> }>()
const emit = defineEmits<{ 'update:visible': [visible: boolean]; created: [id: string] }>()
const { t } = useI18n()
const name = ref('')
const description = ref('')
const fileName = ref('')
const result = ref<DatasetImportResult | null>(null)
const error = ref('')
const reading = ref(false)
const saving = ref(false)
const previewPage = ref(0)
const expandedQuestionId = ref<number | null>(null)
const previewPages = computed(() => Math.max(1, Math.ceil((result.value?.content?.questions.length ?? 0) / 20)))
const visibleQuestions = computed(() => result.value?.content?.questions.slice(previewPage.value * 20, previewPage.value * 20 + 20) ?? [])
const corpusById = computed(() => new Map(result.value?.content?.corpus.map(passage => [passage.id, passage.text]) ?? []))
const metadataIssues = computed(() => evaluationDatasetMetadataIssues(name.value, description.value))
let fileGeneration = 0
const isDirty = (): boolean => props.visible && !!(name.value || description.value || fileName.value)
async function requestClose(): Promise<void> {
  if (saving.value || isDirty() && !await props.confirmDiscard(t('common.unsavedChanges.body'))) return
  emit('update:visible', false)
}

watch(() => props.visible, visible => {
  fileGeneration += 1
  if (!visible) return
  name.value = ''; description.value = ''; fileName.value = ''; result.value = null; error.value = ''; reading.value = false
})
watch(result, () => { previewPage.value = 0; expandedQuestionId.value = null })
watch(previewPage, () => { expandedQuestionId.value = null })

async function readFile(event: Event): Promise<void> {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const generation = ++fileGeneration
  result.value = null
  error.value = ''
  reading.value = true
  fileName.value = file.name
  try {
    if (file.size > DATASET_IMPORT_MAX_BYTES) throw new RangeError(t('evaluation.importSizeLimit'))
    const bytes = await file.arrayBuffer()
    const text = new TextDecoder('utf-8', { fatal: true, ignoreBOM: false }).decode(bytes)
    const extension = file.name.split('.').pop()?.toLocaleLowerCase()
    const parsers = { json: parseDatasetJSON, jsonl: parseDatasetJSONL, csv: parseDatasetCSV }
    if (extension !== 'json' && extension !== 'jsonl' && extension !== 'csv') throw new TypeError(t('evaluation.importUnsupported'))
    const parsed = parsers[extension](text)
    if (generation !== fileGeneration) return
    result.value = parsed
    if (!name.value) name.value = Array.from(file.name.replace(/\.[^.]+$/, '')).slice(0, 200).join('')
  } catch (failure) {
    if (generation === fileGeneration) error.value = failure instanceof Error ? failure.message : JSON.stringify(failure)
  } finally {
    if (generation === fileGeneration) reading.value = false
    input.value = ''
  }
}

function passageText(pid: number): string {
  const text = corpusById.value.get(pid)
  if (text === undefined) throw new TypeError('Import preview references missing passage ' + pid)
  return text
}
function togglePreview(qid: number, event: Event): void {
  if ((event.target as HTMLDetailsElement).open) expandedQuestionId.value = qid
  else if (expandedQuestionId.value === qid) expandedQuestionId.value = null
}

async function saveImport(): Promise<void> {
  if (!result.value?.content || metadataIssues.value.length || saving.value) return
  saving.value = true
  error.value = ''
  try {
    const draft = await createEvaluationDataset({ name: name.value, description: description.value, content: result.value.content })
    emit('update:visible', false)
    emit('created', draft.id)
  } catch (failure) {
    error.value = failure instanceof Error ? failure.message : JSON.stringify(failure)
  } finally { saving.value = false }
}

function downloadJSONTemplate(): void {
  downloadTemplate(datasetJSONTemplate(), 'evaluation-dataset-template.json', 'application/json;charset=utf-8')
}
function downloadCSVTemplate(): void {
  downloadTemplate(datasetCSVTemplate(), 'evaluation-dataset-template.csv', 'text/csv;charset=utf-8')
}
function downloadTemplate(text: string, filename: string, type: string): void {
  const url = URL.createObjectURL(new Blob([text], { type }))
  const link = document.createElement('a')
  link.href = url; link.download = filename; link.click()
  URL.revokeObjectURL(url)
}
defineExpose({ isDirty, isSaving: () => saving.value })
</script>
