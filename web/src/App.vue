<script setup lang="ts">
import type { GlobalThemeOverrides } from 'naive-ui'

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#2563eb',
    primaryColorHover: '#1d4ed8',
    primaryColorPressed: '#1e40af',
    primaryColorSuppl: '#2563eb',
    errorColor: '#dc2626',
    errorColorHover: '#b91c1c',
    errorColorPressed: '#991b1b',
    errorColorSuppl: '#dc2626'
  },
  Button: {
    textColorPrimary: '#ffffff',
    textColorHoverPrimary: '#ffffff',
    textColorPressedPrimary: '#ffffff',
    textColorFocusPrimary: '#ffffff',
    colorPrimary: '#2563eb',
    colorHoverPrimary: '#1d4ed8',
    colorPressedPrimary: '#1e40af',
    borderPrimary: '1px solid #2563eb',
    borderHoverPrimary: '1px solid #1d4ed8',
    borderPressedPrimary: '1px solid #1e40af',
    textColorError: '#ffffff',
    textColorHoverError: '#ffffff',
    textColorPressedError: '#ffffff',
    textColorFocusError: '#ffffff',
    textColorGhostError: '#dc2626',
    textColorGhostHoverError: '#b91c1c',
    textColorGhostPressedError: '#991b1b',
    textColorGhostFocusError: '#b91c1c',
    colorError: '#dc2626',
    colorHoverError: '#b91c1c',
    colorPressedError: '#991b1b',
    colorFocusError: '#b91c1c',
    borderError: '1px solid #dc2626',
    borderHoverError: '1px solid #b91c1c',
    borderPressedError: '1px solid #991b1b',
    borderFocusError: '1px solid #b91c1c'
  }
}
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <RouterView />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
