<script setup lang="ts">
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'
import { computed, onBeforeUnmount, ref } from 'vue'

const lightThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#0969da',
    primaryColorHover: '#0550ae',
    primaryColorPressed: '#0550ae',
    primaryColorSuppl: '#0969da',
    errorColor: '#cf222e',
    errorColorHover: '#a40e26',
    errorColorPressed: '#82071e',
    errorColorSuppl: '#cf222e',
    bodyColor: '#f6f8fa',
    cardColor: '#ffffff',
    modalColor: '#ffffff',
    popoverColor: '#ffffff',
    borderColor: '#d8dee4',
    textColorBase: '#24292f',
    textColor1: '#1f2328',
    textColor2: '#24292f',
    textColor3: '#57606a',
    fontFamily:
      'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    borderRadius: '6px',
    borderRadiusSmall: '6px'
  },
  Button: {
    textColor: '#24292f',
    textColorHover: '#0550ae',
    textColorPressed: '#0550ae',
    textColorFocus: '#0550ae',
    color: '#ffffff',
    colorHover: '#f6f8fa',
    colorPressed: '#f6f8fa',
    colorFocus: '#ffffff',
    border: '1px solid #d8dee4',
    borderHover: '1px solid #0969da',
    borderPressed: '1px solid #0550ae',
    borderFocus: '1px solid #0969da',
    textColorPrimary: '#ffffff',
    textColorHoverPrimary: '#ffffff',
    textColorPressedPrimary: '#ffffff',
    textColorFocusPrimary: '#ffffff',
    colorPrimary: '#0969da',
    colorHoverPrimary: '#0550ae',
    colorPressedPrimary: '#0550ae',
    borderPrimary: '1px solid #0969da',
    borderHoverPrimary: '1px solid #0550ae',
    borderPressedPrimary: '1px solid #0550ae',
    textColorError: '#ffffff',
    textColorHoverError: '#ffffff',
    textColorPressedError: '#ffffff',
    textColorFocusError: '#ffffff',
    textColorGhostError: '#cf222e',
    textColorGhostHoverError: '#b91c1c',
    textColorGhostPressedError: '#991b1b',
    textColorGhostFocusError: '#b91c1c',
    colorError: '#cf222e',
    colorHoverError: '#b91c1c',
    colorPressedError: '#991b1b',
    colorFocusError: '#b91c1c',
    borderError: '1px solid #cf222e',
    borderHoverError: '1px solid #b91c1c',
    borderPressedError: '1px solid #991b1b',
    borderFocusError: '1px solid #b91c1c'
  },
  Input: {
    border: '1px solid #d8dee4',
    borderHover: '1px solid #afb8c1',
    borderFocus: '1px solid #0969da',
    boxShadowFocus: '0 0 0 3px rgba(9, 105, 218, 0.16)',
    color: '#ffffff',
    colorFocus: '#ffffff',
    textColor: '#24292f',
    placeholderColor: '#6e7781'
  },
  Select: {
    peers: {
      InternalSelection: {
        border: '1px solid #d8dee4',
        borderHover: '1px solid #afb8c1',
        borderActive: '1px solid #0969da',
        boxShadowActive: '0 0 0 3px rgba(9, 105, 218, 0.16)',
        color: '#ffffff',
        textColor: '#24292f',
        placeholderColor: '#6e7781'
      }
    }
  },
  Form: {
    labelTextColor: '#24292f',
    feedbackTextColorError: '#cf222e'
  },
  Descriptions: {
    thColor: '#f6f8fa',
    thColorModal: '#f6f8fa',
    thColorPopover: '#f6f8fa',
    thTextColor: '#24292f',
    tdColor: '#ffffff',
    tdColorModal: '#ffffff',
    tdColorPopover: '#ffffff',
    tdTextColor: '#24292f',
    borderColor: '#d8dee4',
    borderColorModal: '#d8dee4',
    borderColorPopover: '#d8dee4'
  },
  Tag: {
    borderRadius: '999px'
  }
}

const darkThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#58a6ff',
    primaryColorHover: '#79c0ff',
    primaryColorPressed: '#388bfd',
    primaryColorSuppl: '#58a6ff',
    errorColor: '#f85149',
    errorColorHover: '#ff7b72',
    errorColorPressed: '#da3633',
    errorColorSuppl: '#f85149',
    bodyColor: '#0d1117',
    cardColor: '#161b22',
    modalColor: '#161b22',
    popoverColor: '#161b22',
    borderColor: '#30363d',
    textColorBase: '#e6edf3',
    textColor1: '#f0f6fc',
    textColor2: '#e6edf3',
    textColor3: '#8b949e',
    fontFamily:
      'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    borderRadius: '6px',
    borderRadiusSmall: '6px'
  },
  Button: {
    textColor: '#e6edf3',
    textColorHover: '#f0f6fc',
    textColorPressed: '#f0f6fc',
    textColorFocus: '#f0f6fc',
    color: '#161b22',
    colorHover: '#21262d',
    colorPressed: '#30363d',
    colorFocus: '#161b22',
    border: '1px solid #30363d',
    borderHover: '1px solid #8b949e',
    borderPressed: '1px solid #8b949e',
    borderFocus: '1px solid #58a6ff',
    textColorPrimary: '#ffffff',
    textColorHoverPrimary: '#ffffff',
    textColorPressedPrimary: '#ffffff',
    textColorFocusPrimary: '#ffffff',
    colorPrimary: '#238636',
    colorHoverPrimary: '#2ea043',
    colorPressedPrimary: '#238636',
    borderPrimary: '1px solid #238636',
    borderHoverPrimary: '1px solid #2ea043',
    borderPressedPrimary: '1px solid #238636',
    textColorError: '#ffffff',
    textColorHoverError: '#ffffff',
    textColorPressedError: '#ffffff',
    textColorFocusError: '#ffffff',
    textColorGhostError: '#f85149',
    textColorGhostHoverError: '#ff7b72',
    textColorGhostPressedError: '#da3633',
    textColorGhostFocusError: '#ff7b72',
    colorError: '#da3633',
    colorHoverError: '#f85149',
    colorPressedError: '#b62324',
    colorFocusError: '#f85149',
    borderError: '1px solid #da3633',
    borderHoverError: '1px solid #f85149',
    borderPressedError: '1px solid #b62324',
    borderFocusError: '1px solid #f85149'
  },
  Input: {
    border: '1px solid #30363d',
    borderHover: '1px solid #8b949e',
    borderFocus: '1px solid #58a6ff',
    boxShadowFocus: '0 0 0 3px rgba(88, 166, 255, 0.22)',
    color: '#0d1117',
    colorFocus: '#0d1117',
    textColor: '#e6edf3',
    placeholderColor: '#8b949e'
  },
  Select: {
    peers: {
      InternalSelection: {
        border: '1px solid #30363d',
        borderHover: '1px solid #8b949e',
        borderActive: '1px solid #58a6ff',
        boxShadowActive: '0 0 0 3px rgba(88, 166, 255, 0.22)',
        color: '#0d1117',
        textColor: '#e6edf3',
        placeholderColor: '#8b949e'
      }
    }
  },
  Form: {
    labelTextColor: '#e6edf3',
    feedbackTextColorError: '#f85149'
  },
  Descriptions: {
    thColor: '#21262d',
    thColorModal: '#21262d',
    thColorPopover: '#21262d',
    thTextColor: '#f0f6fc',
    tdColor: '#161b22',
    tdColorModal: '#161b22',
    tdColorPopover: '#161b22',
    tdTextColor: '#e6edf3',
    borderColor: '#30363d',
    borderColorModal: '#30363d',
    borderColorPopover: '#30363d'
  },
  Tag: {
    borderRadius: '999px'
  }
}

const prefersDark = ref(false)
const colorSchemeQuery =
  typeof window === 'undefined' || !window.matchMedia
    ? undefined
    : window.matchMedia('(prefers-color-scheme: dark)')

if (colorSchemeQuery) {
  prefersDark.value = colorSchemeQuery.matches
  colorSchemeQuery.addEventListener('change', handleColorSchemeChange)
}

const themeOverrides = computed(() => (prefersDark.value ? darkThemeOverrides : lightThemeOverrides))
const naiveTheme = computed(() => (prefersDark.value ? darkTheme : null))

onBeforeUnmount(() => {
  colorSchemeQuery?.removeEventListener('change', handleColorSchemeChange)
})

function handleColorSchemeChange(event: MediaQueryListEvent) {
  prefersDark.value = event.matches
}
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <RouterView />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
