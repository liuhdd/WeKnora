<template>
  <section class="eval-run-detail" data-testid="evaluation-run-detail">
    <div class="eval-section-heading">
      <div><span class="eval-eyebrow">{{ t('evaluation.history') }}</span><h3 class="eval-run-id">{{ run.task.id }}</h3></div>
      <div class="eval-actions"><span class="eval-status" :class="'is-' + taskStatus">{{ t('evaluation.status.' + taskStatus) }}</span>
        <t-button variant="outline" theme="default" size="small" data-testid="evaluation-export" @click="exportRun">
          <template #icon><t-icon name="download" /></template>{{ t('evaluation.export') }}</t-button></div>
    </div>
    <div class="eval-run-meta">
      <div><span>{{ t('evaluation.dataset') }}</span><strong>{{ run.task.dataset_id }}</strong></div>
      <div><span>{{ t('evaluation.version') }}</span><code :title="run.task.dataset_version">{{ run.task.dataset_version_label || t('evaluation.noRecorded') }} · {{ run.task.dataset_version?.slice(0, 12) || t('evaluation.noRecorded') }}</code></div>
      <div><span>{{ t('evaluation.startedAt') }}</span><strong>{{ when(run.task.start_time) }}</strong></div>
      <div><span>{{ t('evaluation.duration') }}</span><strong>{{ duration === null ? t('evaluation.notFinished') : evaluationDuration(duration) }}</strong></div>
    </div>
    <div class="eval-progress"><progress :value="run.task.finished ?? 0" :max="run.task.total || 1" />
      <span>{{ t('evaluation.completed') }} {{ run.task.finished ?? 0 }} / {{ run.task.total ?? 0 }}</span>
      <span v-if="run.task.failed" class="eval-negative">{{ t('evaluation.status.failed') }} {{ run.task.failed }}</span></div>
    <t-alert v-if="run.task.err_msg" theme="error" :title="t('evaluation.executionError')" :message="run.task.err_msg" />

    <section class="eval-panel eval-overview">
      <div class="eval-section-heading"><h3>{{ t('evaluation.qualityMetrics') }}</h3>
        <label class="eval-baseline"><span>{{ t('evaluation.baseline') }}</span>
          <select v-model="baselineId" data-testid="evaluation-baseline" :aria-label="t('evaluation.baseline')" :disabled="!baselines.length || baselineLoading">
            <option value="">{{ t('evaluation.compare') }}</option>
            <option v-for="item in baselines" :key="item.task.id" :value="item.task.id">{{ when(item.task.start_time) }} · {{ item.task.id }}</option>
          </select></label>
      </div>
      <p class="eval-muted">{{ t('evaluation.sameVersionOnly') }}</p>
      <t-alert v-if="baselineError" theme="error" :message="baselineError" />
      <EvaluationMetricGrid :retrieval="run.metric?.retrieval_metrics" :generation="run.metric?.generation_metrics" :baseline="baseline?.metric" />
      <p class="eval-muted eval-metric-hint">{{ t('evaluation.metricHint') }}</p>
    </section>

    <details class="eval-panel eval-parameters" data-testid="evaluation-parameters">
      <summary>{{ t('evaluation.parameters') }}</summary>
      <template v-if="run.params">
        <dl class="eval-parameter-grid">
          <div><dt>{{ t('evaluation.metricVersion') }}</dt><dd>{{ run.task.metric_version || t('evaluation.noRecorded') }}</dd></div>
          <div><dt>{{ t('evaluation.embeddingModel') }}</dt><dd>{{ run.task.embedding_model_id || t('evaluation.noRecorded') }}</dd></div>
          <div><dt>{{ t('evaluation.knowledgeBase') }}</dt><dd>{{ run.task.knowledge_base_id || t('evaluation.noRecorded') }}</dd></div>
          <div><dt>{{ t('evaluation.chatModel') }}</dt><dd>{{ run.params.chat_model_id || t('evaluation.noRecorded') }}</dd></div>
          <div><dt>{{ t('evaluation.rerankModel') }}</dt><dd>{{ run.params.rerank_model_id || t('evaluation.noRecorded') }}</dd></div>
          <div><dt>Embedding Top K</dt><dd>{{ run.params.embedding_top_k ?? t('evaluation.noRecorded') }}</dd></div>
          <div><dt>Rerank Top K</dt><dd>{{ run.params.rerank_top_k ?? t('evaluation.noRecorded') }}</dd></div>
          <div><dt>{{ t('evaluation.vectorThreshold') }}</dt><dd>{{ run.params.vector_threshold ?? t('evaluation.noRecorded') }}</dd></div>
          <div><dt>{{ t('evaluation.rerankThreshold') }}</dt><dd>{{ run.params.rerank_threshold ?? t('evaluation.noRecorded') }}</dd></div>
        </dl>
        <pre>{{ JSON.stringify(run.params, null, 2) }}</pre>
      </template>
      <p v-else class="eval-muted">{{ t('evaluation.noRecorded') }}</p>
    </details>

    <section class="eval-panel eval-results">
      <div class="eval-section-heading"><h3>{{ t('evaluation.questions') }} <span class="eval-muted">{{ run.results?.length ?? 0 }}</span></h3></div>
      <div class="eval-toolbar">
        <t-input v-model="query" data-testid="evaluation-result-search" :placeholder="t('evaluation.searchQuestions')" :aria-label="t('evaluation.searchQuestions')" clearable />
        <select v-model="filter" data-testid="evaluation-question-filter" :aria-label="t('evaluation.questions')">
          <option value="all">{{ t('evaluation.allQuestions') }}</option><option value="failed">{{ t('evaluation.failedQuestions') }}</option>
          <option value="low">{{ t('evaluation.lowScoreQuestions') }}</option>
        </select>
        <template v-if="filter === 'low'">
          <select v-model="scoreKey" data-testid="evaluation-score-metric" :aria-label="t('evaluation.qualityMetrics')">
            <option v-for="key in allMetricKeys" :key="key" :value="key">{{ metricLabels[key] }}</option>
          </select>
          <label class="eval-threshold">{{ t('evaluation.scoreThreshold') }}
            <input v-model.number="threshold" type="number" min="0" max="1" step="0.05" data-testid="evaluation-score-threshold" /></label>
        </template>
      </div>
      <div v-if="visibleQuestions.length" class="eval-split">
        <nav class="eval-question-list" :aria-label="t('evaluation.questions')">
          <button v-for="item in visibleQuestions" :key="item.qid" type="button" class="eval-question-row"
            :class="{ 'is-selected': selected?.qid === item.qid }" :aria-current="selected?.qid === item.qid"
            :data-testid="'evaluation-result-' + item.qid" @click="selectedId = item.qid">
            <div class="eval-question-row-head"><span class="eval-question-id">#{{ item.qid }}</span>
              <span class="eval-status" :class="'is-' + item.status">{{ t('evaluation.status.' + item.status) }}</span></div>
            <strong>{{ item.question }}</strong><span class="eval-question-preview">{{ item.error || item.answer || t('evaluation.noRecorded') }}</span>
            <span v-if="item.metric" class="eval-question-score">Recall {{ item.metric.retrieval_metrics.recall.toFixed(3) }} · ROUGE-L {{ item.metric.generation_metrics.rougel.toFixed(3) }}</span>
          </button>
        </nav>
        <EvaluationQuestionDetail v-if="selected" :result="selected" :baseline="baselineQuestion" />
      </div>
      <div v-else class="eval-empty">{{ t(run.results?.length ? 'evaluation.noMatches' : 'evaluation.noTrace') }}</div>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getEvaluationRun, type EvaluationDetail } from '@/api/evaluation'
import { canCompareEvaluationRuns, evaluationDuration, evaluationRunDuration, filterEvaluationQuestions,
  generationMetricKeys, metricLabels, retrievalMetricKeys, type EvaluationMetricKey, type EvaluationQuestionFilter } from './evaluationPresentation'
import EvaluationMetricGrid from './EvaluationMetricGrid.vue'
import EvaluationQuestionDetail from './EvaluationQuestionDetail.vue'

const props = defineProps<{ run: EvaluationDetail; runs: EvaluationDetail[] }>()
const { t, locale } = useI18n()
const query = ref('')
const filter = ref<EvaluationQuestionFilter>('all')
const scoreKey = ref<EvaluationMetricKey>('recall')
const threshold = ref(0.5)
const selectedId = ref<number | null>(null)
const baselineId = ref('')
const baseline = ref<EvaluationDetail | null>(null)
const baselineLoading = ref(false)
const baselineError = ref('')
const allMetricKeys = [...retrievalMetricKeys, ...generationMetricKeys]
const taskStatus = computed(() => ['pending', 'running', 'success', 'failed'][props.run.task.status])
const duration = computed(() => evaluationRunDuration(props.run))
const baselines = computed(() => props.runs.filter(run => canCompareEvaluationRuns(props.run, run)))
const visibleQuestions = computed(() => filterEvaluationQuestions(props.run.results ?? [], query.value, filter.value, scoreKey.value, threshold.value))
const selected = computed(() => visibleQuestions.value.find(item => item.qid === selectedId.value))
const baselineQuestion = computed(() => baseline.value?.results?.find(item => item.qid === selectedId.value))
const when = (value: string): string => new Date(value).toLocaleString(locale.value)

watch(visibleQuestions, items => {
  if (!items.some(item => item.qid === selectedId.value)) selectedId.value = items[0]?.qid ?? null
}, { immediate: true })

watch(baselineId, async (id, previous, onCleanup) => {
  let active = true
  onCleanup(() => { active = false })
  baseline.value = null
  baselineError.value = ''
  if (!id) return
  baselineLoading.value = true
  try {
    const result = await getEvaluationRun(id)
    if (!canCompareEvaluationRuns(props.run, result)) throw new TypeError('Evaluation comparison requires successful runs with matching dataset and metric versions')
    if (active) baseline.value = result
  } catch (error) {
    if (active) baselineError.value = error instanceof Error ? error.message : JSON.stringify(error)
  } finally {
    if (active) baselineLoading.value = false
  }
})

function exportRun(): void {
  const blob = new Blob([JSON.stringify(props.run, null, 2)], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = props.run.task.id + '.json'
  link.click()
  URL.revokeObjectURL(url)
}
</script>
