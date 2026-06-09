import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AppPagination from './AppPagination.vue'

describe('AppPagination', () => {
  it('shows five page numbers before and after the current page', () => {
    const wrapper = mount(AppPagination, {
      props: {
        page: 10,
        pageSize: 50,
        total: 1000
      }
    })

    expect(wrapper.findAll('.pagination-pages button').map((button) => button.text())).toEqual([
      '5',
      '6',
      '7',
      '8',
      '9',
      '10',
      '11',
      '12',
      '13',
      '14',
      '15'
    ])
    expect(wrapper.find('button[aria-current="page"]').text()).toBe('10')
  })

  it('emits page jumps from the input and page size changes', async () => {
    const wrapper = mount(AppPagination, {
      props: {
        page: 1,
        pageSize: 50,
        total: 1000
      }
    })

    await wrapper.get('input[aria-label="跳转页码"]').setValue('12')
    await wrapper.get('form.pagination-jump').trigger('submit')
    await wrapper.get('select[aria-label="每页条数"]').setValue('100')

    expect(wrapper.emitted('update:page')).toEqual([[12]])
    expect(wrapper.emitted('update:page-size')).toEqual([[100]])
  })

  it('clamps input jumps to the available page range', async () => {
    const wrapper = mount(AppPagination, {
      props: {
        page: 2,
        pageSize: 50,
        total: 75
      }
    })

    await wrapper.get('input[aria-label="跳转页码"]').setValue('99')
    await wrapper.get('form.pagination-jump').trigger('submit')

    expect(wrapper.emitted('update:page')).toEqual(undefined)
    expect((wrapper.get('input[aria-label="跳转页码"]').element as HTMLInputElement).value).toBe('2')
  })
})
