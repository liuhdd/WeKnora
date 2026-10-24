import { ref } from 'vue'
import {
  loadPreference,
  savePreference,
  migratePreferencesIntoUser,
} from './preferenceStorage'

export type ThemeMode = 'light' | 'dark' | 'system'
export type ThemePreset = 'weknora' | 'claude' | 'chatgpt'

const THEME_KEY = 'theme'
const PRESET_KEY = 'theme_preset'

function loadTheme(): ThemeMode {
  const v = loadPreference(THEME_KEY)
  if (v === 'light' || v === 'dark' || v === 'system') return v
  return 'light'
}

function loadPreset(): ThemePreset {
  const value = loadPreference(PRESET_KEY)
  if (value === 'claude' || value === 'chatgpt') return value
  return 'weknora'
}

// 明暗模式与配色独立保存，并在所有入口共享。 / Mode and palette are saved independently and shared across consumers.
const currentTheme = ref<ThemeMode>(loadTheme())
const currentPreset = ref<ThemePreset>(loadPreset())

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** 原生窗口与页面使用同一背景令牌，避免刷新闪动。 / Keep native chrome and page backgrounds aligned to prevent reload flashes. */
function syncWailsNativeChrome(effective: 'light' | 'dark') {
  const bg = 'var(--td-bg-color-page)'
  document.documentElement.style.background = bg
  document.documentElement.style.minHeight = '100%'
  document.documentElement.style.colorScheme = effective === 'dark' ? 'dark' : 'light'
  if (document.body) {
    document.body.style.background = bg
    document.body.style.minHeight = '100%'
  }
  const appEl = document.getElementById('app')
  if (appEl) {
    appEl.style.background = bg
    appEl.style.minHeight = '100%'
  }
  const w = (window as Window & {
    runtime?: {
      WindowSetDarkTheme?: () => void
      WindowSetLightTheme?: () => void
      WindowSetBackgroundColour?: (r: number, g: number, b: number, a: number) => void
    }
  }).runtime
  if (!w?.WindowSetBackgroundColour) return
  const background = getComputedStyle(document.documentElement).backgroundColor
  const channels = /^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/.exec(background)
  if (!channels) throw new TypeError(`Unsupported theme background color: ${background}`)
  if (effective === 'dark') w.WindowSetDarkTheme?.()
  else w.WindowSetLightTheme?.()
  w.WindowSetBackgroundColour(Number(channels[1]), Number(channels[2]), Number(channels[3]), 255)
}

function applyTheme(mode: ThemeMode) {
  const effective = mode === 'system' ? getSystemTheme() : mode
  document.documentElement.setAttribute('theme-mode', effective)
  document.documentElement.setAttribute('data-theme-preset', currentPreset.value)
  syncWailsNativeChrome(effective)
}

export function useTheme() {
  function setTheme(mode: ThemeMode): boolean {
    if (mode !== 'light' && mode !== 'dark' && mode !== 'system') return false
    currentTheme.value = mode
    savePreference(THEME_KEY, mode)
    applyTheme(mode)
    return true
  }

  function setPreset(preset: ThemePreset): boolean {
    if (preset !== 'weknora' && preset !== 'claude' && preset !== 'chatgpt') return false
    currentPreset.value = preset
    savePreference(PRESET_KEY, preset)
    applyTheme(currentTheme.value)
    return true
  }

  return { currentTheme, setTheme, currentPreset, setPreset }
}

/** Call once in main.ts to initialise theme and listen for OS changes. */
export function initTheme() {
  currentTheme.value = loadTheme()
  currentPreset.value = loadPreset()
  applyTheme(currentTheme.value)

  // React to OS theme changes when user chose "system"
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (currentTheme.value === 'system') {
      applyTheme('system')
    }
  })
}

/** Re-read preferences from storage (call after login / logout). */
export function reloadThemeFromStorage() {
  migratePreferencesIntoUser()
  currentTheme.value = loadTheme()
  currentPreset.value = loadPreset()
  applyTheme(currentTheme.value)
}
