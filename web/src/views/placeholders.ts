import { defineComponent, h } from 'vue'

export function placeholderView(title: string) {
  return defineComponent({
    name: `${title.replace(/\s+/g, '')}View`,
    setup() {
      return () =>
        h('section', { class: 'page-section' }, [
          h('h1', { class: 'page-title' }, title),
          h('p', { class: 'page-muted' }, '该模块将在后续阶段实现。')
        ])
    }
  })
}
