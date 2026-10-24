<template>
  <div class="general-settings">
    <div class="section-header">
      <h2>{{ $t('general.title') }}</h2>
      <p class="section-description">{{ $t('general.description') }}</p>
    </div>

    <div class="settings-group">
      <!-- 语言选择 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('language.language') }}</label>
          <p class="desc">{{ $t('language.languageDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-select
            v-model="localLanguage"
            :placeholder="$t('language.selectLanguage')"
            @change="handleLanguageChange"
            style="width: 280px;"
          >
            <t-option value="zh-CN" :label="$t('language.zhCN')">{{ $t('language.zhCN') }}</t-option>
            <t-option value="zh-TW" :label="$t('language.zhTW')">{{ $t('language.zhTW') }}</t-option>
            <t-option value="en-US" :label="$t('language.enUS')">{{ $t('language.enUS') }}</t-option>
            <t-option value="ru-RU" :label="$t('language.ruRU')">{{ $t('language.ruRU') }}</t-option>
            <t-option value="ko-KR" :label="$t('language.koKR')">{{ $t('language.koKR') }}</t-option>
            <t-option value="ja-JP" :label="$t('language.jaJP')">{{ $t('language.jaJP') }}</t-option>
          </t-select>
        </div>
      </div>

      <!-- 主题设置 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('theme.theme') }}</label>
          <p class="desc">{{ $t('theme.themeDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-select
            v-model="localTheme"
            style="width: 280px;"
            :placeholder="$t('theme.selectTheme')"
            @change="handleThemeChange"
          >
            <t-option value="light" :label="$t('theme.light')">{{ $t('theme.light') }}</t-option>
            <t-option value="dark" :label="$t('theme.dark')">{{ $t('theme.dark') }}</t-option>
            <t-option value="system" :label="$t('theme.system')">{{ $t('theme.system') }}</t-option>
          </t-select>
        </div>
      </div>

      <!-- 配色风格与明暗模式独立保存。 / Color presets are saved independently of light/dark mode. -->
      <div class="setting-row setting-row--presets">
        <div class="setting-info">
          <label id="theme-preset-label">{{ $t('theme.preset') }}</label>
          <p class="desc">{{ $t('theme.presetDescription') }}</p>
        </div>
        <div class="theme-presets" role="group" aria-labelledby="theme-preset-label">
          <button
            v-for="preset in presetOptions"
            :key="preset.value"
            type="button"
            class="theme-preset"
            :class="[`theme-preset--${preset.value}`, { 'is-selected': currentPreset === preset.value }]"
            :data-testid="`theme-preset-${preset.value}`"
            :aria-pressed="currentPreset === preset.value"
            @click="setPreset(preset.value)"
          >
            <span class="theme-preset__preview" aria-hidden="true">
              <span class="theme-preset__sidebar"><i /><i /><i /></span>
              <span class="theme-preset__conversation">
                <i class="theme-preset__bubble" />
                <i class="theme-preset__line" />
                <i class="theme-preset__line theme-preset__line--short" />
                <i class="theme-preset__composer"><i /></i>
              </span>
            </span>
            <span class="theme-preset__name">
              {{ preset.label }}
              <span v-if="currentPreset === preset.value" class="theme-preset__check" aria-hidden="true">✓</span>
            </span>
            <span class="theme-preset__description">{{ preset.description }}</span>
          </button>
        </div>
      </div>

      <!-- 界面字体 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('font.uiFont') }}</label>
          <p class="desc">{{ $t('font.uiFontDescription') }}</p>
        </div>
        <div class="setting-control setting-control--stacked">
          <t-select
            v-model="localSansFont"
            style="width: 280px;"
            :placeholder="$t('font.selectFont')"
            @change="handleSansFontChange"
          >
            <t-option
              v-for="opt in sansFontOptions"
              :key="opt.value"
              :value="opt.value"
              :label="opt.label"
            >
              <span :style="{ fontFamily: opt.preview }">{{ opt.label }}</span>
            </t-option>
          </t-select>
          <div class="font-preview" :style="{ fontFamily: currentSansStack }">
            {{ $t('font.sansPreview') }}
          </div>
        </div>
      </div>

      <!-- 代码字体 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('font.monoFont') }}</label>
          <p class="desc">{{ $t('font.monoFontDescription') }}</p>
        </div>
        <div class="setting-control setting-control--stacked">
          <t-select
            v-model="localMonoFont"
            style="width: 280px;"
            :placeholder="$t('font.selectFont')"
            @change="handleMonoFontChange"
          >
            <t-option
              v-for="opt in monoFontOptions"
              :key="opt.value"
              :value="opt.value"
              :label="opt.label"
            >
              <span :style="{ fontFamily: opt.preview }">{{ opt.label }}</span>
            </t-option>
          </t-select>
          <div class="font-preview font-preview--mono" :style="{ fontFamily: currentMonoStack }">
            {{ $t('font.monoPreview') }}
          </div>
        </div>
      </div>

      <!-- 字体大小 -->
      <div class="setting-row">
        <div class="setting-info">
          <label>{{ $t('font.fontSize') }}</label>
          <p class="desc">{{ $t('font.fontSizeDescription') }}</p>
        </div>
        <div class="setting-control">
          <t-radio-group
            v-model="localFontSize"
            @change="handleFontSizeChange"
          >
            <t-radio-button value="small">{{ $t('font.size.small') }}</t-radio-button>
            <t-radio-button value="normal">{{ $t('font.size.normal') }}</t-radio-button>
            <t-radio-button value="large">{{ $t('font.size.large') }}</t-radio-button>
          </t-radio-group>
        </div>
      </div>

      <!-- 自动下载更新开关 (Lite edition only) -->
      <div class="setting-row" v-if="authStore.isLiteMode">
        <div class="setting-info">
          <label>{{ $t('settings.autoCheckUpdate') }}</label>
          <p class="desc">{{ $t('settings.autoCheckUpdateDesc') }}</p>
        </div>
        <div class="setting-control">
          <t-switch
            v-model="isAutoCheckUpdateEnabled"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { useSettingsStore } from '@/stores/settings'
import { useAuthStore } from '@/stores/auth'
import { useTheme, type ThemeMode, type ThemePreset } from '@/composables/useTheme'
import {
  useFont,
  SANS_STACKS,
  MONO_STACKS,
  visibleSansKeys,
  visibleMonoKeys,
  type FontKey,
  type MonoFontKey,
  type FontSizeKey,
} from '@/composables/useFont'

const { t, locale } = useI18n()
const settingsStore = useSettingsStore()
const authStore = useAuthStore()
const { currentTheme, setTheme, currentPreset, setPreset } = useTheme()
const presetOptions = computed<{ value: ThemePreset; label: string; description: string }[]>(() => [
  { value: 'weknora', label: 'WeKnora', description: t('theme.weknoraDescription') },
  { value: 'claude', label: 'Claude', description: t('theme.claudeDescription') },
  { value: 'chatgpt', label: 'ChatGPT', description: t('theme.chatgptDescription') },
])
const {
  currentSans,
  currentMono,
  currentSize,
  setSansFont,
  setMonoFont,
  setFontSize,
} = useFont()

// 本地状态
const localLanguage = ref('zh-CN')
const localTheme = ref<ThemeMode>(currentTheme.value)
const localSansFont = ref<FontKey>(currentSans.value)
const localMonoFont = ref<MonoFontKey>(currentMono.value)
const localFontSize = ref<FontSizeKey>(currentSize.value)

// Keep the form in sync if preferences change externally (e.g. on user switch).
watch(currentTheme, (val) => { localTheme.value = val })
watch(currentSans, (val) => { localSansFont.value = val })
watch(currentMono, (val) => { localMonoFont.value = val })
watch(currentSize, (val) => { localFontSize.value = val })

const sansFontOptions = computed<{ value: FontKey; label: string; preview: string }[]>(() =>
  visibleSansKeys().map((key) => ({
    value: key,
    label: t(`font.sans.${key}`),
    preview: SANS_STACKS[key],
  })),
)

const monoFontOptions = computed<{ value: MonoFontKey; label: string; preview: string }[]>(() =>
  visibleMonoKeys().map((key) => ({
    value: key,
    label: t(`font.mono.${key}`),
    preview: MONO_STACKS[key],
  })),
)

// Live preview stacks, driven by the local form refs so the preview row
// updates immediately on selection — even before handleSansFontChange
// commits the choice to the global store and writes the CSS variable.
const currentSansStack = computed(() => SANS_STACKS[localSansFont.value] ?? SANS_STACKS.system)
const currentMonoStack = computed(() => MONO_STACKS[localMonoFont.value] ?? MONO_STACKS.system)

// 自动检查更新状态
const isAutoCheckUpdateEnabled = computed({
  get: () => settingsStore.isAutoCheckUpdateEnabled,
  set: (val) => {
    settingsStore.toggleAutoCheckUpdate(val)
    if (val) {
      // @ts-ignore
      if (window.go && window.go.main && window.go.main.App && window.go.main.App.AutoCheckForUpdates) {
        // @ts-ignore
        window.go.main.App.AutoCheckForUpdates()
      }
    }
  }
})

// 初始化加载
onMounted(() => {
  // 从 localStorage 加载语言设置
  const savedLocale = localStorage.getItem('locale')
  if (savedLocale) {
    localLanguage.value = savedLocale
    locale.value = savedLocale
  } else {
    localLanguage.value = locale.value
  }
})

// 处理语言变化
const handleLanguageChange = () => {
  locale.value = localLanguage.value
  localStorage.setItem('locale', localLanguage.value)
  MessagePlugin.success(t('language.languageSaved'))
    }

// 处理主题变化
const handleThemeChange = (val: ThemeMode) => {
  if (!setTheme(val)) {
    // Setter rejected the value (validation guard); roll the form back to
    // the canonical state so the UI doesn't drift.
    localTheme.value = currentTheme.value
    return
  }
}

// 处理字体变化
const handleSansFontChange = (val: FontKey) => {
  if (!setSansFont(val)) {
    localSansFont.value = currentSans.value
    return
  }
}

const handleMonoFontChange = (val: MonoFontKey) => {
  if (!setMonoFont(val)) {
    localMonoFont.value = currentMono.value
    return
  }
}

const handleFontSizeChange = (val: FontSizeKey) => {
  if (!setFontSize(val)) {
    localFontSize.value = currentSize.value
    return
  }
}
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/settings-section.less';

.general-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  .setting-row();
}

.setting-info {
  .setting-info();
}

.setting-control {
  .setting-control();
}

.setting-row--presets {
  flex-direction: column;
  gap: 16px;

  .setting-info { max-width: 100%; }
}

.theme-presets {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
}

.theme-preset {
  --preview-page: #ffffff;
  --preview-sidebar: #f2f5f3;
  --preview-accent: #07c05f;
  --preview-text: #bdc7c1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color var(--app-motion-fast), background-color var(--app-motion-fast);

  &:hover { border-color: var(--td-brand-color); }
  &:focus-visible { outline: 2px solid var(--td-brand-color); outline-offset: 3px; }
  &.is-selected {
    border-color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }

  &--claude {
    --preview-page: #f7f5ef;
    --preview-sidebar: #eae7de;
    --preview-accent: #b75b3c;
    --preview-text: #b8b1a3;
  }

  &--chatgpt {
    --preview-page: #ffffff;
    --preview-sidebar: #f0f0f0;
    --preview-accent: #212121;
    --preview-text: #b8b8b8;
  }

  &__preview {
    display: flex;
    width: 100%;
    height: 80px;
    overflow: hidden;
    border: 1px solid var(--td-component-stroke);
    border-radius: var(--app-radius-sm);
    background: var(--preview-page);
  }

  &__sidebar {
    display: flex;
    flex-direction: column;
    gap: 7px;
    width: 24%;
    padding: 13px 6px;
    background: var(--preview-sidebar);

    i { height: 3px; border-radius: var(--app-radius-pill); background: var(--preview-text); }
    i:first-child { background: var(--preview-accent); }
  }

  &__conversation { flex: 1; display: flex; flex-direction: column; gap: 5px; padding: 12px 10px 8px; }
  &__bubble { width: 46%; height: 9px; align-self: flex-end; border-radius: var(--app-radius-pill); background: var(--preview-sidebar); }
  &__line { height: 3px; width: 85%; background: var(--preview-text); border-radius: var(--app-radius-pill); }
  &__line--short { width: 60%; }
  &__composer {
    display: flex;
    justify-content: flex-end;
    margin-top: auto;
    padding: 3px;
    border: 1px solid var(--preview-text);
    border-radius: var(--app-radius-pill);

    i { width: 9px; height: 9px; background: var(--preview-accent); border-radius: 50%; }
  }

  &__name { display: flex; justify-content: space-between; gap: 8px; font-size: var(--app-text-base); font-weight: 600; }
  &__check { color: var(--td-brand-color); }
  &__description { color: var(--td-text-color-secondary); font-size: var(--app-text-sm); line-height: 1.5; }
}

:global(html[theme-mode='dark'] .theme-preset) {
  --preview-page: #242424;
  --preview-sidebar: #181818;
  --preview-text: #666666;
}

:global(html[theme-mode='dark'] .theme-preset--claude) {
  --preview-page: #292823;
  --preview-sidebar: #201f1c;
  --preview-accent: #e3a68b;
  --preview-text: #787369;
}

:global(html[theme-mode='dark'] .theme-preset--chatgpt) {
  --preview-page: #212121;
  --preview-sidebar: #171717;
  --preview-accent: #ececec;
}

@media (max-width: 560px) {
  .theme-presets { grid-template-columns: 1fr; }
}

// When a font picker is rendered, stack the select on top of a live
// preview line so the user can verify their choice without hunting for
// an API Info page or a code block.
.setting-control--stacked {
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.font-preview {
  width: 280px;
  padding: 8px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--td-radius-medium);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: var(--app-text-base);
  line-height: 1.4;
  text-align: left;
  box-sizing: border-box;

  &--mono {
    // Harden the preview against wrap-around for long monospace samples.
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
