<template>
  <article class="eval-question-detail" data-testid="evaluation-question-detail">
    <div class="eval-section-heading"><span class="eval-eyebrow">{{ t('evaluation.questionId') }} #{{ result.qid }}</span>
      <span class="eval-status" :class="'is-' + result.status">{{ t('evaluation.status.' + result.status) }}</span>
    </div>
    <h3>{{ result.question }}</h3>
    <div class="eval-inline-meta">
      <span>{{ t('evaluation.duration') }} {{ result.status === 'pending' || result.status === 'running' ? t('evaluation.notFinished') : evaluationDuration(result.duration_ms) }}</span>
      <span v-if="result.usage">{{ t('evaluation.tokens') }} {{ result.usage.total_tokens }}
        ({{ t('evaluation.inputTokens') }} {{ result.usage.prompt_tokens }} / {{ t('evaluation.outputTokens') }} {{ result.usage.completion_tokens }})</span>
    </div>
    <t-alert v-if="result.error" theme="error" :title="t('evaluation.executionError')" :message="result.error" />
    <div class="eval-answer-grid">
      <section class="eval-answer"><h4>{{ t('evaluation.referenceAnswer') }}</h4><p>{{ result.reference_answer || t('evaluation.noRecorded') }}</p></section>
      <section class="eval-answer eval-answer--generated" data-testid="evaluation-final-answer"><h4>{{ t('evaluation.finalAnswer') }}</h4>
        <p>{{ result.answer || t('evaluation.noRecorded') }}</p></section>
      <section v-if="baseline" class="eval-answer"><h4>{{ t('evaluation.baseline') }} · {{ t('evaluation.finalAnswer') }}</h4>
        <p>{{ baseline.answer || t('evaluation.noRecorded') }}</p></section>
    </div>
    <section data-testid="evaluation-reference-evidence"><h3>{{ t('evaluation.evidence') }}</h3>
      <div v-for="(passage, index) in result.passages" :key="index" class="eval-evidence">
        <span class="eval-eyebrow">PID {{ result.pids?.[index] ?? t('evaluation.noRecorded') }}</span><p>{{ passage }}</p>
      </div>
      <p v-if="!result.passages?.length" class="eval-muted">{{ t('evaluation.noRecorded') }}</p>
    </section>
    <section><h3>{{ t('evaluation.qualityMetrics') }}</h3><EvaluationMetricGrid :retrieval="result.metric?.retrieval_metrics"
      :generation="result.metric?.generation_metrics" :baseline="baseline?.metric" /></section>
    <section class="eval-trace" data-testid="evaluation-trace">
      <h3>{{ t('evaluation.process') }} <span class="eval-muted">{{ result.steps?.length ?? 0 }}</span></h3>
      <p class="eval-muted">{{ t('evaluation.documentScoreHint') }}</p>
      <ol v-if="result.steps?.length" class="eval-step-list">
        <li v-for="(step, index) in result.steps" :key="step.id" :data-testid="'evaluation-step-' + step.id">
          <details class="eval-step" :open="index === 0">
            <summary><span class="eval-step-number" :class="{ 'is-failed': step.status === 'failed' }">{{ index + 1 }}</span>
              <strong :title="step.id">{{ stageLabel(step.id, step.name) }}</strong><span class="eval-status" :class="'is-' + step.status">{{ t('evaluation.status.' + step.status) }}</span>
              <time>{{ evaluationDuration(step.duration_ms) }}</time></summary>
            <div class="eval-step-body">
              <t-alert v-if="step.error" theme="error" :message="step.error" />
              <div class="eval-answer-grid">
                <section><h4>{{ t('evaluation.stepInput') }}</h4><pre>{{ step.input || t('evaluation.noRecorded') }}</pre></section>
                <section><h4>{{ t('evaluation.stepOutput') }}</h4><pre>{{ step.output || t('evaluation.noRecorded') }}</pre></section>
              </div>
              <div v-if="step.documents?.length" class="eval-documents">
                <details v-for="(document, rank) in step.documents" :key="document.id + '-' + rank" class="eval-document">
                  <summary><span>#{{ rank + 1 }}</span><code>{{ document.id }}</code>
                    <strong>{{ t('evaluation.documentScore') }} {{ document.score.toFixed(4) }}</strong></summary>
                  <div><p class="eval-muted">{{ t('evaluation.source') }} · {{ document.source || t('evaluation.noRecorded') }}</p><p>{{ document.content }}</p></div>
                </details>
              </div>
              <EvaluationMetricGrid v-if="step.metric" :retrieval="step.metric_scope === 'retrieval' ? step.metric.retrieval_metrics : undefined"
                :generation="step.metric_scope === 'generation' ? step.metric.generation_metrics : undefined" :baseline="undefined" />
              <p v-else class="eval-muted">{{ t('evaluation.noMetrics') }}</p>
            </div>
          </details>
        </li>
      </ol>
      <p v-else class="eval-muted">{{ t('evaluation.noTrace') }}</p>
    </section>
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { EvaluationQuestionResult } from '@/api/evaluation'
import { evaluationDuration } from './evaluationPresentation'
import EvaluationMetricGrid from './EvaluationMetricGrid.vue'

defineProps<{ result: EvaluationQuestionResult; baseline: EvaluationQuestionResult | undefined }>()
const { t, te } = useI18n()
const stageLabel = (id: string, name: string): string => {
  const key = 'evaluation.stages.' + id
  return te(key) ? t(key) : name
}
</script>
