<template>
  <div v-if="retrieval || generation" class="eval-metric-groups" data-testid="evaluation-quality-metrics">
    <section v-for="group in groups" :key="group.label">
      <h4>{{ t(group.label) }}</h4>
      <div class="eval-metric-grid">
        <div v-for="key in group.keys" :key="key" class="eval-metric" :data-testid="'evaluation-metric-' + key">
          <span>{{ metricLabels[key] }}</span>
          <strong>{{ currentScore(key)?.toFixed(3) }}</strong>
          <small v-if="baseline" class="eval-metric-comparison">
            {{ t('evaluation.baseline') }} {{ metricValue(baseline, key)?.toFixed(3) }}
            <b :class="delta(key) >= 0 ? 'eval-positive' : 'eval-negative'">Δ {{ delta(key) > 0 ? '+' : '' }}{{ delta(key).toFixed(3) }}</b>
          </small>
        </div>
      </div>
    </section>
  </div>
  <p v-else class="eval-muted">{{ t('evaluation.noMetrics') }}</p>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvaluationMetrics, GenerationMetrics, RetrievalMetrics } from '@/api/evaluation'
import { generationMetricKeys, metricLabels, metricValue, retrievalMetricKeys, type EvaluationMetricKey } from './evaluationPresentation'

const props = defineProps<{
  retrieval: RetrievalMetrics | undefined
  generation: GenerationMetrics | undefined
  baseline: EvaluationMetrics | undefined
}>()
const { t } = useI18n()
const groups = computed(() => [
  ...(props.retrieval ? [{ label: 'evaluation.retrievalMetrics', keys: retrievalMetricKeys }] : []),
  ...(props.generation ? [{ label: 'evaluation.generationMetrics', keys: generationMetricKeys }] : []),
])
const currentScore = (key: EvaluationMetricKey): number | undefined => {
  if (props.retrieval && key in props.retrieval) return props.retrieval[key as keyof RetrievalMetrics]
  return props.generation?.[key as keyof GenerationMetrics]
}
const delta = (key: EvaluationMetricKey): number => {
  const current = currentScore(key)
  const baseline = metricValue(props.baseline, key)
  return current !== undefined && baseline !== undefined ? current - baseline : Number.NaN
}
</script>
