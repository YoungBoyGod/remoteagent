import { Node, mergeAttributes } from '@tiptap/core'
import { VueNodeViewRenderer } from '@tiptap/vue-3'
import DrawingNode from './DrawingNode.vue'

export const DrawingBlock = Node.create({
  name: 'drawingBlock',
  group: 'block',
  atom: true,

  addAttributes() {
    return {
      data: {
        default: '',
        parseHTML: (el) => el.getAttribute('data-drawing') || '',
        renderHTML: (attrs) => ({ 'data-drawing': attrs.data }),
      },
      width: { default: '100%' },
      height: { default: '360' },
    }
  },

  parseHTML() {
    return [{ tag: 'div[data-type="drawing"]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['div', mergeAttributes(HTMLAttributes, { 'data-type': 'drawing' })]
  },

  addNodeView() {
    return VueNodeViewRenderer(DrawingNode)
  },
})
