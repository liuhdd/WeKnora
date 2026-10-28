<template>
  <section class="eval-dataset" data-testid="evaluation-dataset">
    <div class="eval-section-heading">
      <div><h3>{{ dataset.name }}</h3><p class="eval-muted">{{ t('evaluation.datasetReadonly') }}</p></div>
      <span class="eval-version" :title="dataset.version">{{ t('evaluation.version') }} · {{ dataset.version.slice(0, 12) }}</span>
    </div>
    <div class="eval-toolbar">
      <t-input v-model="query" data-testid="evaluation-dataset-search" :placeholder="t('evaluation.searchQuestions')"
        :aria-label="t('evaluation.searchQuestions')" clearable><template #prefix-icon><t-icon name="search" /></template></t-input>
      <span class="eval-muted">{{ visibleQuestions.length }} / {{ dataset.total }} {{ t('evaluation.questions') }}</span>
    </div>
    <div v-if="visibleQuestions.length" class="eval-split">
      <nav class="eval-question-list" :aria-label="t('evaluation.questions')">
        <button v-for="item in visibleQuestions" :key="item.qid" type="button" class="eval-question-row"
          :class="{ 'is-selected': selected?.qid === item.qid }" :aria-current="selected?.qid === item.qid"
          :data-testid="'evaluation-dataset-question-' + item.qid" @click="selectedId = item.qid">
          <span class="eval-question-id">#{{ item.qid }}</span><strong>{{ item.question }}</strong>
          <span class="eval-question-preview">{{ item.answer || t('evaluation.noRecorded') }}</span>
        </button>
      </nav>
      <article v-if="selected" class="eval-question-detail">
        <span class="eval-eyebrow">{{ t('evaluation.questionId') }} #{{ selected.qid }}</span>
        <h3>{{ selected.question }}</h3>
        <section class="eval-answer"><h4>{{ t('evaluation.referenceAnswer') }}</h4><p>{{ selected.answer || t('evaluation.noRecorded') }}</p></section>
        <section><h4>{{ t('evaluation.evidence') }} · {{ selected.passages.length }}</h4>
          <div v-for="(passage, index) in selected.passages" :key="index" class="eval-evidence">
            <span class="eval-eyebrow">PID {{ selected.pids[index] }}</span><p>{{ passage }}</p>
          </div>
          <p v-if="!selected.passages.length" class="eval-muted">{{ t('evaluation.noRecorded') }}</p>
        </section>
      </article>
    </div>
    <div v-else class="eval-empty">{{ t(query ? 'evaluation.noMatches' : 'evaluation.noQuestions') }}</div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EvaluationDataset } from '@/api/evaluation'

const props = defineProps<{ dataset: EvaluationDataset }>()
const { t } = useI18n()
const query = ref('')
const selectedId = ref<number | null>(null)
const visibleQuestions = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  return props.dataset.items.filter(item => !needle ||
    (item.qid + ' ' + item.question + ' ' + item.answer).toLocaleLowerCase().includes(needle))
})
const selected = computed(() => visibleQuestions.value.find(item => item.qid === selectedId.value))
watch(visibleQuestions, items => {
  if (!items.some(item => item.qid === selectedId.value)) selectedId.value = items[0]?.qid ?? null
}, { immediate: true })
</script>
