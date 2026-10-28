<template>
  <section class="eval-dataset-manager" data-testid="dataset-manager">
    <div class="eval-toolbar eval-dataset-controls">
      <label>{{ t('evaluation.dataset') }}<select :value="selectedId" data-testid="evaluation-dataset-select" @change="selectDataset">
        <option v-for="item in datasets" :key="item.id" :value="item.id">{{ item.name }} · {{ item.read_only ? t('evaluation.readOnly') : item.published_version || t('evaluation.notPublished') }}</option>
      </select></label>
      <div v-if="canManage" class="eval-actions"><t-button size="small" variant="outline" data-testid="dataset-new" @click="openNew">{{ t('evaluation.newDataset') }}</t-button>
        <t-button size="small" variant="outline" data-testid="dataset-import-open" @click="openImport">{{ t('evaluation.importDataset') }}</t-button></div>
    </div>
    <t-alert v-if="error" theme="error" :message="error" />
    <div v-if="loading" class="eval-empty"><t-loading /></div>
    <template v-else-if="summary">
      <div class="eval-section-heading"><div><h3>{{ summary.name }}</h3><p class="eval-muted">{{ summary.description }}</p></div>
        <span v-if="summary.read_only" class="eval-version">{{ t('evaluation.readOnly') }}</span>
        <span v-else class="eval-version">{{ t('evaluation.revision') }} {{ draft?.revision ?? 0 }} · {{ t(dirty ? 'evaluation.unsaved' : 'evaluation.saved') }}</span>
      </div>
      <div class="eval-toolbar">
        <button v-if="!summary.read_only" type="button" class="eval-mode-button" :class="{ 'is-active': mode === 'draft' }" data-testid="dataset-draft-tab" @click="mode = 'draft'">{{ t('evaluation.draft') }}</button>
        <label>{{ t('evaluation.versionHistory') }}<select :value="mode === 'version' ? versionLabel : ''" data-testid="dataset-version-select" @change="selectVersion">
          <option value="">{{ t('evaluation.selectPublishedVersion') }}</option>
          <option v-for="version in versions" :key="version.version" :value="version.version">{{ version.version }} · {{ version.total }} · {{ version.created_at ? when(version.created_at) : '' }}</option>
        </select></label>
        <t-button v-if="canManage && summary.read_only && preview" size="small" variant="outline" data-testid="dataset-copy" @click="openCopy">{{ t('evaluation.copyDataset') }}</t-button>
        <t-button v-if="mode === 'version' && preview" size="small" variant="outline" data-testid="dataset-version-export" @click="exportVersion">{{ t('evaluation.exportDatasetJSON') }}</t-button>
        <t-button v-if="canManage && !summary.read_only && mode === 'version' && preview" size="small" variant="outline" data-testid="dataset-restore" :disabled="!!busy" @click="restoreVersion">{{ t('evaluation.restoreDraft') }}</t-button>
      </div>
      <template v-if="mode === 'draft' && draft">
        <p class="eval-muted">{{ t('evaluation.draftHint') }}</p>
        <p class="eval-muted">{{ t('evaluation.datasetLimits') }}</p>
        <template v-if="canManage">
          <div class="eval-dataset-metadata eval-edit-fields"><label>{{ t('evaluation.datasetName') }}<input v-model="name" type="text" data-testid="dataset-name" :disabled="!!busy" /></label>
            <label>{{ t('evaluation.datasetDescription') }}<textarea v-model="description" rows="2" data-testid="dataset-description" :disabled="!!busy" /></label></div>
          <div class="eval-toolbar"><t-button data-testid="dataset-save-draft" :loading="busy === 'save'" :disabled="!!busy || !dirty" @click="saveDraft">{{ t('evaluation.saveDraft') }}</t-button>
            <t-button variant="outline" data-testid="dataset-publish" :loading="busy === 'publish'" :disabled="!!busy || dirty" :title="t('evaluation.saveBeforePublish')" @click="publishDraft">{{ t('evaluation.publishVersion') }}</t-button>
            <t-button theme="default" variant="text" data-testid="dataset-draft-export" @click="exportDraft">{{ t('evaluation.exportDatasetJSON') }}</t-button></div>
          <EvaluationDatasetIssues :issues="issues" />
          <EvaluationDatasetEditor :key="selectedId" :content="content" :disabled="!!busy" @update:content="updateContent" @remove-question="removeQuestion" @remove-passage="removePassage" />
        </template>
        <p v-else class="eval-muted">{{ t('evaluation.adminOnly') }}</p>
      </template>
      <template v-if="mode === 'version'">
        <t-loading v-if="versionLoading" size="small" />
        <EvaluationDataset v-else-if="preview" :key="preview.id + preview.published_version" :dataset="preview" />
        <p v-else class="eval-empty">{{ t('evaluation.noPublishedVersions') }}</p>
      </template>
    </template>

    <EvaluationDatasetImport ref="importPanel" v-model:visible="importVisible" :confirm-discard="confirmAction" @created="id => emit('changed', id)" />
    <t-dialog :visible="newVisible" :header="t('evaluation.newDataset')" :footer="false" width="540px" top="6vh" dialog-class-name="evaluation-create-dialog"
      @update:visible="(value: boolean) => !value && closeNew()">
      <form class="evaluation-create-form" data-testid="dataset-create-form" @submit.prevent="createDataset">
        <t-alert v-if="newError" theme="error" :message="newError" />
        <label>{{ t('evaluation.datasetName') }}<input v-model="newName" type="text" data-testid="dataset-create-name" required :disabled="newSaving" /></label>
        <label>{{ t('evaluation.datasetDescription') }}<textarea v-model="newDescription" rows="2" data-testid="dataset-create-description" :disabled="newSaving" /></label>
        <EvaluationDatasetIssues :issues="newMetadataIssues" />
        <p v-if="copyContent" class="eval-muted">{{ t('evaluation.copyCompleteCorpus', { questions: copyContent.questions.length, passages: copyContent.corpus.length }) }}</p>
        <div class="eval-actions"><t-button theme="default" variant="outline" data-testid="dataset-create-cancel" :disabled="newSaving" @click="closeNew">{{ t('common.cancel') }}</t-button>
          <t-button type="submit" data-testid="dataset-create-save" :loading="newSaving" :disabled="newMetadataIssues.length > 0">{{ t('evaluation.createDraft') }}</t-button></div>
      </form>
    </t-dialog>
    <t-dialog :visible="confirmVisible" :header="t('evaluation.confirmDatasetAction')" :footer="false" width="480px" top="10vh" dialog-class-name="evaluation-create-dialog"
      @update:visible="(value: boolean) => !value && resolveConfirmation(false)">
      <div class="evaluation-create-form"><p>{{ confirmBody }}</p><div class="eval-actions">
        <t-button theme="default" variant="outline" data-testid="dataset-confirm-cancel" @click="resolveConfirmation(false)">{{ t('common.cancel') }}</t-button>
        <t-button data-testid="dataset-confirm-action" @click="resolveConfirmation(true)">{{ t('common.confirm') }}</t-button>
      </div></div>
    </t-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getEvaluationDataset, type EvaluationDataset as EvaluationDatasetPreview, type EvaluationDatasetSummary } from '@/api/evaluation'
import { createEvaluationDataset, evaluationContentFromVersion, evaluationDatasetMetadataIssues, getEvaluationDatasetDraft,
  listEvaluationDatasetVersions, publishEvaluationDataset, saveEvaluationDatasetDraft,
  type EvaluationDatasetDraft, type EvaluationDatasetVersion } from '@/api/evaluation-datasets'
import { registerUnsavedChanges } from '@/utils/unsavedChanges'
import { cloneDatasetContent, validateDatasetDraft, validateDatasetPublication, type DatasetValidationIssue, type EvaluationDatasetContent } from './evaluationDatasetImport'
import EvaluationDataset from './EvaluationDataset.vue'
import EvaluationDatasetEditor from './EvaluationDatasetEditor.vue'
import EvaluationDatasetImport from './EvaluationDatasetImport.vue'
import EvaluationDatasetIssues from './EvaluationDatasetIssues.vue'

const props = defineProps<{ datasets: EvaluationDatasetSummary[]; selectedId: string; canManage: boolean }>()
const emit = defineEmits<{ select: [id: string]; changed: [id: string] }>()
const { t, locale } = useI18n()
const summary = computed(() => props.datasets.find(item => item.id === props.selectedId))
const draft = ref<EvaluationDatasetDraft | null>(null)
const content = ref<EvaluationDatasetContent>({ questions: [], corpus: [] })
const name = ref('')
const description = ref('')
const versions = ref<EvaluationDatasetVersion[]>([])
const mode = ref<'draft' | 'version'>('draft')
const versionLabel = ref('')
const preview = ref<EvaluationDatasetPreview | null>(null)
const loading = ref(false)
const versionLoading = ref(false)
const error = ref('')
const issues = ref<DatasetValidationIssue[]>([])
const busy = ref<'' | 'save' | 'publish' | 'restore'>('')
const importVisible = ref(false)
const importPanel = ref<{ isDirty: () => boolean; isSaving: () => boolean } | null>(null)
const newVisible = ref(false)
const newName = ref('')
const newDescription = ref('')
const newError = ref('')
const newSaving = ref(false)
const copyContent = ref<EvaluationDatasetContent | null>(null)
const confirmVisible = ref(false)
const confirmBody = ref('')
let confirmation: ((answer: boolean) => void) | null = null
let loadGeneration = 0
let versionGeneration = 0
const dirty = computed(() => !!draft.value && JSON.stringify({ name: name.value, description: description.value, content: content.value }) !==
  JSON.stringify({ name: draft.value.name, description: draft.value.description, content: draft.value.content }))
const newMetadataIssues = computed(() => evaluationDatasetMetadataIssues(newName.value, newDescription.value))
const when = (value: string): string => new Date(value).toLocaleString(locale.value)

function acceptDraft(value: EvaluationDatasetDraft): void {
  draft.value = value; name.value = value.name; description.value = value.description
  content.value = cloneDatasetContent(value.content); issues.value = []
}
function confirmAction(body: string): Promise<boolean> {
  if (confirmation) return Promise.resolve(false)
  confirmBody.value = body; confirmVisible.value = true
  return new Promise(resolve => { confirmation = resolve })
}
function resolveConfirmation(answer: boolean): void {
  confirmVisible.value = false
  const resolve = confirmation; confirmation = null; resolve?.(answer)
}
async function protectUnsaved(): Promise<boolean> {
  if (busy.value || newSaving.value || importPanel.value?.isSaving()) return false
  if (!dirty.value && !newVisible.value && !importPanel.value?.isDirty()) return true
  if (!await confirmAction(t('common.unsavedChanges.body'))) return false
  if (draft.value) acceptDraft(draft.value)
  newVisible.value = false; importVisible.value = false
  return true
}
async function selectDataset(event: Event): Promise<void> {
  const select = event.target as HTMLSelectElement
  const id = select.value
  select.value = props.selectedId
  if (id !== props.selectedId && await protectUnsaved()) emit('select', id)
}
async function loadSelection(id: string): Promise<void> {
  const generation = ++loadGeneration
  versionGeneration += 1
  loading.value = true; error.value = ''; preview.value = null; versionLabel.value = ''; draft.value = null
  if (!id || !summary.value) { loading.value = false; return }
  try {
    const [availableVersions, currentDraft] = await Promise.all([
      listEvaluationDatasetVersions(id), summary.value.read_only ? Promise.resolve(null) : getEvaluationDatasetDraft(id),
    ])
    if (generation !== loadGeneration) return
    versions.value = availableVersions
    if (currentDraft) { acceptDraft(currentDraft); mode.value = 'draft' }
    else {
      mode.value = 'version'
      const version = availableVersions[0]?.version ?? summary.value.published_version
      if (version) { versionLabel.value = version; await loadVersion(version) }
    }
  } catch (failure) {
    if (generation === loadGeneration) error.value = failure instanceof Error ? failure.message : JSON.stringify(failure)
  } finally { if (generation === loadGeneration) loading.value = false }
}
async function loadVersion(version: string): Promise<void> {
  const generation = ++versionGeneration
  versionLoading.value = true; error.value = ''; preview.value = null
  try {
    const result = await getEvaluationDataset(props.selectedId, version)
    if (generation === versionGeneration) preview.value = result
  } catch (failure) {
    if (generation === versionGeneration) error.value = failure instanceof Error ? failure.message : JSON.stringify(failure)
  } finally { if (generation === versionGeneration) versionLoading.value = false }
}
function selectVersion(event: Event): void {
  const version = (event.target as HTMLSelectElement).value
  if (!version) return
  mode.value = 'version'; versionLabel.value = version; void loadVersion(version)
}
async function refresh(): Promise<void> {
  if (await protectUnsaved()) await loadSelection(props.selectedId)
}
async function openNew(): Promise<void> {
  if (!await protectUnsaved()) return
  if (draft.value && dirty.value) acceptDraft(draft.value)
  newName.value = ''; newDescription.value = ''; copyContent.value = null; newError.value = ''; newVisible.value = true
}
async function openCopy(): Promise<void> {
  if (!preview.value) return
  newName.value = preview.value.name + ' ' + t('evaluation.copySuffix'); newDescription.value = preview.value.description
  copyContent.value = evaluationContentFromVersion(preview.value); newError.value = ''; newVisible.value = true
}
async function openImport(): Promise<void> {
  if (!await protectUnsaved()) return
  if (draft.value && dirty.value) acceptDraft(draft.value)
  importVisible.value = true
}
async function closeNew(): Promise<void> {
  if (newSaving.value) return
  if ((newName.value || newDescription.value) && !await confirmAction(t('common.unsavedChanges.body'))) return
  newVisible.value = false
}
async function createDataset(): Promise<void> {
  if (newSaving.value || newMetadataIssues.value.length) return
  newSaving.value = true; newError.value = ''
  try {
    const result = await createEvaluationDataset({ name: newName.value, description: newDescription.value,
      content: copyContent.value ? cloneDatasetContent(copyContent.value) : { questions: [], corpus: [] } })
    newVisible.value = false; emit('changed', result.id)
  } catch (failure) { newError.value = failure instanceof Error ? failure.message : JSON.stringify(failure) }
  finally { newSaving.value = false }
}
async function saveDraft(): Promise<void> {
  if (!draft.value || busy.value) return
  issues.value = [...evaluationDatasetMetadataIssues(name.value, description.value), ...validateDatasetDraft(content.value)]
  if (issues.value.length) return
  busy.value = 'save'; error.value = ''
  try {
    const result = await saveEvaluationDatasetDraft(props.selectedId, { revision: draft.value.revision, name: name.value,
      description: description.value, content: cloneDatasetContent(content.value) })
    acceptDraft(result); emit('changed', result.id)
  } catch (failure) { error.value = failure instanceof Error ? failure.message : JSON.stringify(failure) }
  finally { busy.value = '' }
}
async function publishDraft(): Promise<void> {
  if (!draft.value || dirty.value || busy.value) return
  issues.value = [...evaluationDatasetMetadataIssues(name.value, description.value), ...validateDatasetPublication(content.value)]
  if (issues.value.length || !await confirmAction(t('evaluation.publishConfirm'))) return
  busy.value = 'publish'; error.value = ''
  try {
    await publishEvaluationDataset(props.selectedId, draft.value.revision)
    const [updated, available] = await Promise.all([getEvaluationDatasetDraft(props.selectedId), listEvaluationDatasetVersions(props.selectedId)])
    acceptDraft(updated); versions.value = available; emit('changed', props.selectedId)
  } catch (failure) { error.value = failure instanceof Error ? failure.message : JSON.stringify(failure) }
  finally { busy.value = '' }
}
async function restoreVersion(): Promise<void> {
  if (!draft.value || !preview.value || busy.value || !await confirmAction(t('evaluation.restoreConfirm'))) return
  busy.value = 'restore'; error.value = ''
  try {
    const restored = await saveEvaluationDatasetDraft(props.selectedId, { revision: draft.value.revision, name: preview.value.name,
      description: preview.value.description, content: evaluationContentFromVersion(preview.value) })
    acceptDraft(restored); mode.value = 'draft'; emit('changed', props.selectedId)
  } catch (failure) { error.value = failure instanceof Error ? failure.message : JSON.stringify(failure) }
  finally { busy.value = '' }
}
async function removeQuestion(qid: number): Promise<void> {
  if (busy.value) return
  if (await confirmAction(t('evaluation.deleteQuestionConfirm', { id: qid })) && !busy.value) content.value = { ...content.value, questions: content.value.questions.filter(item => item.qid !== qid) }
}
async function removePassage(pid: number): Promise<void> {
  if (busy.value) return
  const references = content.value.questions.filter(question => question.pids.includes(pid))
  if (references.length) { error.value = t('evaluation.unlinkBeforeDelete'); return }
  if (await confirmAction(t('evaluation.deletePassageConfirm', { id: pid })) && !busy.value && !content.value.questions.some(question => question.pids.includes(pid))) {
    content.value = { ...content.value, corpus: content.value.corpus.filter(item => item.id !== pid) }
  }
}
function updateContent(value: EvaluationDatasetContent): void { if (!busy.value) content.value = value }
function downloadContent(value: EvaluationDatasetContent, label: string): void {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = props.selectedId + '-' + label + '.json'; link.click(); URL.revokeObjectURL(url)
}
function exportDraft(): void { downloadContent(content.value, 'draft') }
function exportVersion(): void { if (preview.value) downloadContent(evaluationContentFromVersion(preview.value), versionLabel.value) }
function beforeUnload(event: BeforeUnloadEvent): void {
  if (dirty.value || newVisible.value || importPanel.value?.isDirty() || busy.value) { event.preventDefault(); event.returnValue = '' }
}
watch(() => props.selectedId, id => { void loadSelection(id) }, { immediate: true })
onBeforeRouteLeave(() => protectUnsaved())
onBeforeRouteUpdate((to, from) => to.params.taskId !== from.params.taskId ? protectUnsaved() : true)
let unregister: (() => void) | null = null
onMounted(() => { unregister = registerUnsavedChanges(() => dirty.value || newVisible.value || importPanel.value?.isDirty() === true); window.addEventListener('beforeunload', beforeUnload) })
onBeforeUnmount(() => { loadGeneration += 1; versionGeneration += 1; unregister?.(); window.removeEventListener('beforeunload', beforeUnload); resolveConfirmation(false) })
defineExpose({ refresh, protectUnsaved })
</script>
