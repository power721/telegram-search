import { describe, expect, it } from 'vitest'
import resourceTableSource from './ResourceTable.vue?raw'

describe('ResourceTable', () => {
  it('fills the available horizontal space', () => {
    expect(resourceTableSource).toMatch(/\.resource-table\s*\{[\s\S]*\bwidth:\s*100%;/)
  })
})
