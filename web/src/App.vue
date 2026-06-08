<script setup lang="ts">
import type { GlobalThemeOverrides } from 'naive-ui'

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#2563eb',
    primaryColorHover: '#1d4ed8',
    primaryColorPressed: '#1e40af',
    primaryColorSuppl: '#2563eb'
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
    borderPressedPrimary: '1px solid #1e40af'
  }
}
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-message-provider>
      <RouterView />
    </n-message-provider>
  </n-config-provider>
</template>
