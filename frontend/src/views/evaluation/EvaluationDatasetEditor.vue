<template>
  <section class="eval-dataset-editor" data-testid="dataset-editor">
    <fieldset class="eval-editor-fieldset" :disabled="disabled">
    <div class="eval-toolbar">
      <div class="eval-editor-tabs" role="tablist" :aria-label="t('evaluation.dataset')">
        <button type="button" role="tab" :aria-selected="section === 'questions'" data-testid="dataset-editor-questions"
          :class="{ 'is-active': section === 'questions' }" @click="section = 'questions'">{{ t('evaluation.questions') }} · {{ content.questions.length }}</button>
        <button type="button" role="tab" :aria-selected="section === 'corpus'" data-testid="dataset-editor-corpus"
          :class="{ 'is-active': section === 'corpus' }" @click="section = 'corpus'">{{ t('evaluation.datasetCorpus') }} · {{ content.corpus.length }}</button>
      </div>
      <t-button v-if="section === 'questions'" size="small" variant="outline" data-testid="dataset-add-question" :disabled="disabled || content.questions.length >= DATASET_MAX_QUESTIONS" @click="addQuestion">{{ t('evaluation.addQuestion') }}</t-button>
      <t-button size="small" variant="outline" data-testid="dataset-add-passage" :disabled="disabled || content.corpus.length >= DATASET_MAX_PASSAGES" @click="addPassage">{{ t('evaluation.addPassage') }}</t-button>
    </div>

    <div v-if="section === 'questions'" class="eval-split">
      <nav class="eval-question-list" :aria-label="t('evaluation.questions')">
        <button v-for="item in content.questions" :key="item.qid" type="button" class="eval-question-row" :class="{ 'is-selected': question?.qid === item.qid }"
          :data-testid="'dataset-edit-question-' + item.qid" @click="questionId = item.qid">
          <span class="eval-question-id">#{{ item.qid }}</span><strong>{{ item.question || t('evaluation.untitledQuestion') }}</strong>
          <span class="eval-question-preview">{{ item.answer || t('evaluation.noRecorded') }}</span>
        </button>
        <p v-if="!content.questions.length" class="eval-empty">{{ t('evaluation.noQuestions') }}</p>
      </nav>
      <article v-if="question" class="eval-question-detail eval-edit-fields">
        <div class="eval-section-heading"><span class="eval-eyebrow">QID {{ question.qid }}</span>
          <t-button theme="danger" variant="text" size="small" data-testid="dataset-remove-question" @click="emit('remove-question', question.qid)">{{ t('common.delete') }}</t-button></div>
        <label>{{ t('evaluation.question') }}<textarea :value="question.question" rows="3" data-testid="dataset-question-text"
          @input="updateQuestionText(question.qid, ($event.target as HTMLTextAreaElement).value)" /></label>
        <label>{{ t('evaluation.referenceAnswer') }}<textarea :value="question.answer" rows="5" data-testid="dataset-question-answer"
          @input="updateQuestionAnswer(question.qid, ($event.target as HTMLTextAreaElement).value)" /></label>
        <section><h4>{{ t('evaluation.evidence') }} · {{ question.pids.length }}</h4>
          <p class="eval-muted">{{ t('evaluation.selectSharedEvidence') }}</p>
          <div class="eval-selected-pids"><span v-for="pid in question.pids" :key="pid">PID {{ pid }}</span></div>
          <t-input v-model="evidenceQuery" data-testid="dataset-evidence-search" :placeholder="t('evaluation.searchEvidence')" clearable />
          <label v-for="passage in visiblePassages" :key="passage.id" class="eval-evidence-choice">
            <input type="checkbox" :checked="question.pids.includes(passage.id)" :data-testid="'dataset-link-passage-' + passage.id"
              @change="toggleQuestionEvidence(question.qid, passage.id)" />
            <span><b>PID {{ passage.id }}</b><small>{{ passage.text || t('evaluation.emptyPassage') }}</small></span>
          </label>
          <div class="eval-passage-pagination"><button type="button" :disabled="evidencePage === 0" data-testid="dataset-evidence-previous" @click="evidencePage -= 1">‹</button>
            <span>{{ evidencePage + 1 }} / {{ passagePages }}</span><button type="button" :disabled="evidencePage + 1 >= passagePages" data-testid="dataset-evidence-next" @click="evidencePage += 1">›</button></div>
        </section>
      </article>
      <p v-else class="eval-empty">{{ t('evaluation.addQuestionHint') }}</p>
    </div>

    <div v-else>
      <p class="eval-muted">{{ t('evaluation.sharedEvidenceHint') }}</p>
      <div class="eval-toolbar"><t-input v-model="evidenceQuery" data-testid="dataset-corpus-search" :placeholder="t('evaluation.searchEvidence')" clearable /></div>
      <div class="eval-split">
        <nav class="eval-question-list" :aria-label="t('evaluation.datasetCorpus')">
          <button v-for="item in visiblePassages" :key="item.id" type="button" class="eval-question-row" :class="{ 'is-selected': passage?.id === item.id }"
            :data-testid="'dataset-edit-passage-' + item.id" @click="passageId = item.id">
            <span class="eval-question-id">PID {{ item.id }} · {{ t('evaluation.evidenceReferences', { count: referenceCount(item.id) }) }}</span>
            <span class="eval-question-preview">{{ item.text || t('evaluation.emptyPassage') }}</span>
          </button>
          <div class="eval-passage-pagination"><button type="button" :disabled="evidencePage === 0" data-testid="dataset-corpus-previous" @click="evidencePage -= 1">‹</button>
            <span>{{ evidencePage + 1 }} / {{ passagePages }}</span><button type="button" :disabled="evidencePage + 1 >= passagePages" data-testid="dataset-corpus-next" @click="evidencePage += 1">›</button></div>
        </nav>
        <article v-if="passage" class="eval-question-detail eval-edit-fields">
          <div class="eval-section-heading"><span class="eval-eyebrow">PID {{ passage.id }}</span>
            <t-button theme="danger" variant="text" size="small" data-testid="dataset-remove-passage" :disabled="referenceCount(passage.id) > 0"
              :title="t('evaluation.unlinkBeforeDelete')" @click="emit('remove-passage', passage.id)">{{ t('common.delete') }}</t-button></div>
          <t-alert v-if="referenceCount(passage.id)" theme="warning" :message="t('evaluation.sharedPassageWarning', { count: referenceCount(passage.id) })" />
          <label>{{ t('evaluation.evidence') }}<textarea :value="passage.text" rows="12" data-testid="dataset-passage-text"
            @input="updatePassageText(passage.id, ($event.target as HTMLTextAreaElement).value)" /></label>
          <p class="eval-muted">{{ t('evaluation.referencedQuestions') }} · {{ referencingQuestions(passage.id).map(item => '#' + item.qid).join(', ') || t('evaluation.noReferences') }}</p>
        </article>
        <p v-else class="eval-empty">{{ t('evaluation.addPassageHint') }}</p>
      </div>
    </div>
    </fieldset>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { DATASET_MAX_PASSAGES, DATASET_MAX_QUESTIONS, type EvaluationDatasetContent } from './evaluationDatasetImport'

const props = defineProps<{ content: EvaluationDatasetContent; disabled: boolean }>()
const emit = defineEmits<{ 'update:content': [content: EvaluationDatasetContent]; 'remove-question': [qid: number]; 'remove-passage': [pid: number] }>()
const { t } = useI18n()
const section = ref<'questions' | 'corpus'>('questions')
const questionId = ref<number | null>(null)
const passageId = ref<number | null>(null)
const evidenceQuery = ref('')
const evidencePage = ref(0)
const question = computed(() => props.content.questions.find(item => item.qid === questionId.value))
const passage = computed(() => props.content.corpus.find(item => item.id === passageId.value))
const matchingPassages = computed(() => {
  const query = evidenceQuery.value.trim().toLocaleLowerCase()
  return props.content.corpus.filter(item => !query || (item.id + ' ' + item.text).toLocaleLowerCase().includes(query))
})
const passagePages = computed(() => Math.max(1, Math.ceil(matchingPassages.value.length / 20)))
const visiblePassages = computed(() => matchingPassages.value.slice(evidencePage.value * 20, evidencePage.value * 20 + 20))
const referencingQuestions = (pid: number) => props.content.questions.filter(item => item.pids.includes(pid))
const referenceCount = (pid: number): number => referencingQuestions(pid).length

watch(() => props.content.questions, items => {
  if (!items.some(item => item.qid === questionId.value)) questionId.value = items[0]?.qid ?? null
}, { immediate: true })
watch(() => props.content.corpus, items => {
  if (!items.some(item => item.id === passageId.value)) passageId.value = items[0]?.id ?? null
}, { immediate: true })
watch(evidenceQuery, () => { evidencePage.value = 0 })
watch(passagePages, pages => { evidencePage.value = Math.min(evidencePage.value, pages - 1) })

function nextIdentifier(ids: number[]): number {
  const used = new Set(ids)
  let id = 0
  while (used.has(id)) id += 1
  return id
}
function addQuestion(): void {
  if (props.disabled || props.content.questions.length >= DATASET_MAX_QUESTIONS) return
  const qid = nextIdentifier(props.content.questions.map(item => item.qid))
  emit('update:content', { questions: [...props.content.questions, { qid, question: '', answer: '', pids: [] }], corpus: props.content.corpus })
  questionId.value = qid
  section.value = 'questions'
}
function addPassage(): void {
  if (props.disabled || props.content.corpus.length >= DATASET_MAX_PASSAGES) return
  const id = nextIdentifier(props.content.corpus.map(item => item.id))
  emit('update:content', { questions: props.content.questions, corpus: [...props.content.corpus, { id, text: '' }] })
  passageId.value = id
  evidenceQuery.value = ''
  evidencePage.value = Math.floor(props.content.corpus.length / 20)
  section.value = 'corpus'
}
function updateQuestionText(qid: number, text: string): void {
  if (props.disabled) return
  emit('update:content', { ...props.content, questions: props.content.questions.map(item => item.qid === qid ? { ...item, question: text } : item) })
}
function updateQuestionAnswer(qid: number, answer: string): void {
  if (props.disabled) return
  emit('update:content', { ...props.content, questions: props.content.questions.map(item => item.qid === qid ? { ...item, answer } : item) })
}
function updatePassageText(pid: number, text: string): void {
  if (props.disabled) return
  emit('update:content', { ...props.content, corpus: props.content.corpus.map(item => item.id === pid ? { ...item, text } : item) })
}
function toggleQuestionEvidence(qid: number, pid: number): void {
  if (props.disabled) return
  emit('update:content', { ...props.content, questions: props.content.questions.map(item => item.qid === qid ? {
    ...item, pids: item.pids.includes(pid) ? item.pids.filter(value => value !== pid) : [...item.pids, pid],
  } : item) })
}
</script>
