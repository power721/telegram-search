import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ResourceFilters from './ResourceFilters.vue'

describe('ResourceFilters', () => {
  it('enables search on the channel dropdown', () => {
    const wrapper = mount(ResourceFilters, {
      props: {
        keyword: '',
        category: '',
        channelId: '',
        channelOptions: [
          { label: '全部频道', value: '' },
          { label: 'Movies (@movies)', value: 7 }
        ],
        'onUpdate:keyword': (value: string) => wrapper.setProps({ keyword: value }),
        'onUpdate:category': (value: string) => wrapper.setProps({ category: value }),
        'onUpdate:channelId': (value: string | number) => wrapper.setProps({ channelId: value })
      },
      global: {
        stubs: {
          NInput: {
            props: ['value'],
            emits: ['update:value'],
            template: '<input v-bind="$attrs" :value="value" @input="$emit(\'update:value\', $event.target.value)" />'
          },
          NSelect: {
            props: {
              value: [String, Number],
              options: Array,
              filterable: Boolean
            },
            emits: ['update:value'],
            template:
              '<select v-bind="$attrs" :data-filterable="filterable ? \'true\' : \'false\'" :value="value" @change="$emit(\'update:value\', $event.target.value)"><option v-for="option in options" :key="String(option.value)" :value="option.value">{{ option.label }}</option></select>'
          },
          NButton: {
            emits: ['click'],
            template: '<button v-bind="$attrs" @click="$emit(\'click\', $event)"><slot /></button>'
          }
        }
      }
    })

    expect(wrapper.get('#resource-channel').attributes('data-filterable')).toBe('true')
  })
})
