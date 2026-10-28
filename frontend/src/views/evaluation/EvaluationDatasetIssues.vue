<template>
  <section v-if="issues.length" class="eval-validation" data-testid="dataset-validation-errors" role="alert">
    <strong>{{ t('evaluation.validationErrors', { count: issues.length }) }}</strong>
    <ul><li v-for="(issue, index) in issues" :key="issue.path + issue.code + index"><code>{{ issue.path }}</code> {{ issue.message }}</li></ul>
  </section>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { DatasetValidationIssue } from './evaluationDatasetImport'
defineProps<{ issues: DatasetValidationIssue[] }>()
const { t } = useI18n()
</script>
