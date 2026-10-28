<template>
  <main class="evaluation-page" data-testid="evaluation-workbench">
    <header class="eval-page-header" style="--wails-draggable: drag">
      <div class="eval-page-title"><h2><t-icon name="chart-bar" size="24px" />{{ t('evaluation.title') }}</h2>
        <div class="eval-actions" style="--wails-draggable: no-drag">
          <t-button theme="default" variant="outline" data-testid="evaluation-refresh" :loading="historyLoading || detailLoading" @click="refresh">
            <template #icon><t-icon name="refresh" /></template>{{ t('evaluation.refresh') }}</t-button>
          <t-button v-if="canStart" data-testid="evaluation-new-run" @click="openCreate">
            <template #icon><t-icon name="add" /></template>{{ t('evaluation.newRun') }}</t-button>
        </div>
      </div>
      <p class="eval-muted">{{ t('evaluation.description') }}</p>
    </header>

    <div class="eval-page-content">
      <template v-if="taskId">
        <button type="button" class="eval-back" data-testid="evaluation-back" @click="backToHistory">
          <t-icon name="arrow-left" />{{ t('evaluation.back') }}</button>
        <t-alert v-if="detailError" theme="error" :message="detailError" />
        <div v-if="detailLoading && !selectedRun" class="eval-empty"><t-loading /></div>
        <EvaluationRunDetail v-else-if="selectedRun" :key="selectedRun.task.id" :run="selectedRun" :runs="runs" />
      </template>
      <template v-else>
        <div class="eval-tabbar" role="tablist" :aria-label="t('evaluation.title')">
          <button type="button" role="tab" :aria-selected="tab === 'history'" :class="{ 'is-active': tab === 'history' }"
            data-testid="evaluation-history-tab" @click="tab = 'history'">{{ t('evaluation.history') }}</button>
          <button type="button" role="tab" :aria-selected="tab === 'datasets'" :class="{ 'is-active': tab === 'datasets' }"
            data-testid="evaluation-datasets-tab" @click="tab = 'datasets'">{{ t('evaluation.datasets') }}<span>{{ datasets.length }}</span></button>
        </div>

        <section v-show="tab === 'history'" role="tabpanel" class="eval-history" data-testid="evaluation-history">
          <t-alert v-if="historyError" theme="error" :message="historyError" />
          <div class="eval-stat-grid">
            <div><span>{{ t('evaluation.runCount') }}</span><strong>{{ runs.length }}<small> / {{ historyTotal }}</small></strong></div>
            <div><span>{{ t('evaluation.successCount') }}</span><strong>{{ runs.filter(run => run.task.status === 2).length }}</strong></div>
            <div><span>{{ t('evaluation.runningCount') }}</span><strong>{{ runs.filter(run => run.task.status < 2).length }}</strong></div>
          </div>
          <div class="eval-toolbar"><t-input v-model="historyQuery" data-testid="evaluation-history-search"
            :placeholder="t('evaluation.searchRuns')" :aria-label="t('evaluation.searchRuns')" clearable>
            <template #prefix-icon><t-icon name="search" /></template></t-input></div>
          <div v-if="historyLoading && !runs.length" class="eval-empty"><t-loading /></div>
          <div v-else-if="!visibleRuns.length" class="eval-empty"><t-icon name="chart-bar" size="32px" />
            <p>{{ t(historyQuery ? 'common.noResult' : 'evaluation.historyEmpty') }}</p>
            <p v-if="!canStart" class="eval-muted">{{ t('evaluation.adminOnly') }}</p></div>
          <div v-else class="eval-table-wrap">
            <table class="eval-table"><thead><tr><th>{{ t('evaluation.history') }}</th><th>{{ t('evaluation.dataset') }}</th>
              <th>{{ t('evaluation.progress') }}</th><th>Recall</th><th>ROUGE-L</th><th>{{ t('evaluation.duration') }}</th></tr></thead>
              <tbody><tr v-for="run in visibleRuns" :key="run.task.id" :data-testid="'evaluation-history-' + run.task.id">
                <td><button type="button" class="eval-run-link" :data-testid="'evaluation-open-' + run.task.id"
                  @click="openRun(run.task.id)"><strong>{{ when(run.task.start_time) }}</strong><span>{{ run.task.id }}</span></button>
                  <span class="eval-status" :class="'is-' + statusKey(run.task.status)">{{ t('evaluation.status.' + statusKey(run.task.status)) }}</span></td>
                <td><strong>{{ run.task.dataset_id }}</strong><code :title="run.task.dataset_version">{{ run.task.dataset_version_label ? run.task.dataset_version_label + ' · ' : '' }}{{ run.task.dataset_version?.slice(0, 10) || t('evaluation.noRecorded') }}</code></td>
                <td>{{ run.task.finished ?? 0 }} / {{ run.task.total ?? 0 }}<span v-if="run.task.failed" class="eval-negative"> · {{ run.task.failed }} {{ t('evaluation.status.failed') }}</span></td>
                <td>{{ run.metric?.retrieval_metrics.recall.toFixed(3) ?? '—' }}</td><td>{{ run.metric?.generation_metrics.rougel.toFixed(3) ?? '—' }}</td>
                <td>{{ durationLabel(run) }}</td>
              </tr></tbody>
            </table>
          </div>
          <div v-if="runs.length < historyTotal" class="eval-load-more"><t-button variant="outline" theme="default" :loading="historyLoading"
            data-testid="evaluation-load-more" @click="loadHistory(historyPage + 1)">{{ t('evaluation.loadMore') }}</t-button></div>
        </section>

        <section v-show="tab === 'datasets'" role="tabpanel" class="eval-datasets">
          <t-alert v-if="datasetError" theme="error" :message="datasetError" />
          <EvaluationDatasetManager ref="datasetManager" :datasets="datasets" :selected-id="datasetId" :can-manage="canStart"
            @select="datasetId = $event" @changed="datasetChanged" />
        </section>
      </template>
    </div>

    <t-dialog v-model:visible="createVisible" :header="t('evaluation.newRun')" :footer="false" width="540px" top="6vh"
      dialog-class-name="evaluation-create-dialog" destroy-on-close>
      <form class="evaluation-create-form" data-testid="evaluation-create-form" @submit.prevent="createRun">
        <p class="eval-muted">{{ t('evaluation.createHint') }}</p>
        <t-alert v-if="createError" theme="error" :message="createError" />
        <label>{{ t('evaluation.dataset') }}<select v-model="createDatasetId" data-testid="evaluation-create-dataset" required>
          <option v-for="item in publishedDatasets" :key="item.id" :value="item.id">{{ item.name }} · {{ item.total }}</option></select></label>
        <label>{{ t('evaluation.version') }}<select v-model="createVersion" data-testid="evaluation-create-version" :disabled="versionsLoading" required>
          <option value="">{{ t('evaluation.selectPublishedVersion') }}</option><option v-for="version in createVersions" :key="version.version" :value="version.version">{{ version.version }} · {{ version.total }}</option></select></label>
        <p v-if="!publishedDatasets.length">{{ t('evaluation.publishBeforeRun') }}</p>
        <label>{{ t('evaluation.knowledgeBase') }}<select v-model="knowledgeBaseId" data-testid="evaluation-create-kb">
          <option value="">{{ t('evaluation.systemDefault') }}</option><option v-for="item in knowledgeBases" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
        <label>{{ t('evaluation.chatModel') }}<select v-model="chatModelId" data-testid="evaluation-create-model">
          <option value="">{{ t('evaluation.systemDefault') }}</option><option v-for="item in chatModels" :key="item.id" :value="item.id">{{ item.display_name || item.name }}</option></select></label>
        <label>{{ t('evaluation.rerankModel') }}<select v-model="rerankModelId" data-testid="evaluation-create-rerank">
          <option value="">{{ t('evaluation.systemDefault') }}</option><option v-for="item in rerankModels" :key="item.id" :value="item.id">{{ item.display_name || item.name }}</option></select></label>
        <div class="eval-actions"><t-button theme="default" variant="outline" :disabled="creating" @click="createVisible = false">{{ t('common.cancel') }}</t-button>
          <t-button type="submit" :loading="creating || resourcesLoading || versionsLoading" :disabled="!createDatasetId || !createVersion || !resourcesReady" data-testid="evaluation-start">{{ t('evaluation.start') }}</t-button></div>
      </form>
    </t-dialog>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { getEvaluationRun, listEvaluationDatasets, listEvaluationRuns, startEvaluation,
  evaluationRequest, type EvaluationDatasetSummary, type EvaluationDetail } from '@/api/evaluation'
import { listEvaluationDatasetVersions, type EvaluationDatasetVersion } from '@/api/evaluation-datasets'
import { listKnowledgeBases } from '@/api/knowledge-base'
import { listModels, type ModelConfig } from '@/api/model'
import type { KnowledgeBaseInfo } from '@/api/auth'
import { evaluationDuration, evaluationRunDuration } from './evaluationPresentation'
import EvaluationDatasetManager from './EvaluationDatasetManager.vue'
import EvaluationRunDetail from './EvaluationRunDetail.vue'

const { t, locale } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const canStart = computed(() => auth.hasRole('admin'))
const taskId = computed(() => typeof route.params.taskId === 'string' ? route.params.taskId : '')
const tab = ref<'history' | 'datasets'>('history')
const datasets = ref<EvaluationDatasetSummary[]>([])
const datasetId = ref('')
const datasetError = ref('')
const datasetManager = ref<{ refresh: () => Promise<void>; protectUnsaved: () => Promise<boolean> } | null>(null)
const runs = ref<EvaluationDetail[]>([])
const historyTotal = ref(0)
const historyPage = ref(1)
const historyQuery = ref('')
const historyLoading = ref(false)
const historyError = ref('')
const selectedRun = ref<EvaluationDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailRefresh = ref(0)
const createVisible = ref(false)
const creating = ref(false)
const resourcesLoading = ref(false)
const resourcesReady = ref(false)
const createError = ref('')
const createDatasetId = ref('')
const createVersion = ref('')
const createVersions = ref<EvaluationDatasetVersion[]>([])
const versionsLoading = ref(false)
const publishedDatasets = computed(() => datasets.value.filter(item => !!item.published_version))
const knowledgeBaseId = ref('')
const chatModelId = ref('')
const rerankModelId = ref('')
const knowledgeBases = ref<KnowledgeBaseInfo[]>([])
const models = ref<ModelConfig[]>([])
const chatModels = computed(() => models.value.filter(model => model.type === 'KnowledgeQA' && model.id))
const rerankModels = computed(() => models.value.filter(model => model.type === 'Rerank' && model.id))
const visibleRuns = computed(() => {
  const needle = historyQuery.value.trim().toLocaleLowerCase()
  return runs.value.filter(run => !needle || (run.task.id + ' ' + run.task.dataset_id).toLocaleLowerCase().includes(needle))
})
const statusKey = (status: number): string => ['pending', 'running', 'success', 'failed'][status]
const when = (value: string): string => new Date(value).toLocaleString(locale.value)
const durationLabel = (run: EvaluationDetail): string => {
  const duration = evaluationRunDuration(run)
  return duration === null ? t('evaluation.notFinished') : evaluationDuration(duration)
}
let disposed = false
let historyTimer: ReturnType<typeof setTimeout> | undefined

async function loadHistory(page: number): Promise<void> {
  clearTimeout(historyTimer)
  historyLoading.value = true
  historyError.value = ''
  try {
    const result = await listEvaluationRuns(page, 20)
    if (disposed) return
    runs.value = page === 1 ? result.items : [...runs.value, ...result.items]
    historyTotal.value = result.total
    historyPage.value = page
    if (!taskId.value && tab.value === 'history' && runs.value.some(run => run.task.status < 2)) {
      historyTimer = setTimeout(() => { void loadHistory(1) }, 3000)
    }
  } catch (error) {
    if (!disposed) historyError.value = error instanceof Error ? error.message : JSON.stringify(error)
  } finally {
    if (!disposed) historyLoading.value = false
  }
}

async function loadDatasets(): Promise<void> {
  datasetError.value = ''
  try {
    const result = await listEvaluationDatasets()
    if (disposed) return
    datasets.value = result
    if (!result.some(item => item.id === datasetId.value)) datasetId.value = result[0]?.id ?? ''
  } catch (error) {
    if (!disposed) datasetError.value = error instanceof Error ? error.message : JSON.stringify(error)
  }
}

watch([createDatasetId, createVisible], async ([id, visible], previous, onCleanup) => {
  let active = true
  onCleanup(() => { active = false })
  createVersion.value = ''; createVersions.value = []
  versionsLoading.value = false
  if (!id || !visible) return
  versionsLoading.value = true
  try {
    const result = await listEvaluationDatasetVersions(id)
    if (active) {
      createVersions.value = result
      createVersion.value = datasets.value.find(item => item.id === id)?.published_version ?? result[0]?.version ?? ''
    }
  } catch (error) {
    if (active) createError.value = error instanceof Error ? error.message : JSON.stringify(error)
  } finally {
    if (active) versionsLoading.value = false
  }
})

// 切换评测或离开页面时停止旧轮询，避免旧结果覆盖当前页面。 Stop stale polling before it can overwrite another run.
watch([taskId, detailRefresh], ([id], previous, onCleanup) => {
  let active = true
  let timer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => { active = false; clearTimeout(timer) })
  selectedRun.value = null
  detailError.value = ''
  if (!id) return
  const load = async (): Promise<void> => {
    detailLoading.value = true
    try {
      const result = await getEvaluationRun(id)
      if (!active) return
      selectedRun.value = result
      if (result.task.status < 2) timer = setTimeout(() => { void load() }, 2000)
    } catch (error) {
      if (active) detailError.value = error instanceof Error ? error.message : JSON.stringify(error)
    } finally {
      if (active) detailLoading.value = false
    }
  }
  void load()
}, { immediate: true })

watch([tab, taskId], () => {
  clearTimeout(historyTimer)
  if (!taskId.value && tab.value === 'history') void loadHistory(1)
})

function openRun(id: string): void {
  void router.push('/platform/evaluations/' + encodeURIComponent(id))
}

function backToHistory(): void {
  tab.value = 'history'
  void router.push('/platform/evaluations')
}

function refresh(): void {
  if (taskId.value) detailRefresh.value += 1
  void loadHistory(1)
  void loadDatasets()
  if (tab.value === 'datasets') void datasetManager.value?.refresh()
}

async function datasetChanged(id: string): Promise<void> {
  await loadDatasets()
  datasetId.value = id
}

async function openCreate(): Promise<void> {
  if (datasetManager.value && !await datasetManager.value.protectUnsaved()) return
  createVisible.value = true
  createError.value = ''
  createDatasetId.value = publishedDatasets.value.some(item => item.id === datasetId.value) ? datasetId.value : publishedDatasets.value[0]?.id ?? ''
  resourcesLoading.value = true
  resourcesReady.value = false
  try {
    const [bases, availableModels] = await evaluationRequest(Promise.all([listKnowledgeBases({ creator: 'all' }), listModels('')]))
    const response: { success: boolean; data: KnowledgeBaseInfo[] } = bases
    if (response.success !== true || !Array.isArray(response.data) ||
        !response.data.every(item => typeof item.id === 'string' && typeof item.name === 'string')) {
      throw new TypeError('Invalid knowledge base response: successful array of id/name entries is required')
    }
    knowledgeBases.value = response.data
    models.value = availableModels
    resourcesReady.value = true
  } catch (error) {
    createError.value = error instanceof Error ? error.message : JSON.stringify(error)
  } finally {
    resourcesLoading.value = false
  }
}

async function createRun(): Promise<void> {
  if (!canStart.value || !createDatasetId.value || !createVersion.value || creating.value || versionsLoading.value || !resourcesReady.value) return
  creating.value = true
  createError.value = ''
  try {
    const task = await startEvaluation({ dataset_id: createDatasetId.value, dataset_version: createVersion.value, knowledge_base_id: knowledgeBaseId.value,
      chat_id: chatModelId.value, rerank_id: rerankModelId.value })
    createVisible.value = false
    openRun(task.id)
    void loadHistory(1)
  } catch (error) {
    createError.value = error instanceof Error ? error.message : JSON.stringify(error)
  } finally {
    creating.value = false
  }
}

onMounted(() => { void loadHistory(1); void loadDatasets() })
onBeforeUnmount(() => { disposed = true; clearTimeout(historyTimer) })
</script>

<style lang="less" src="./evaluation.less"></style>
