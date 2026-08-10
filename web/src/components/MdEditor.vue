<template>
  <div class="md-editor">
    <div class="md-toolbar">
      <el-button-group>
        <el-button size="small" title="标题" @click="wrapLine('## ')">H</el-button>
        <el-button size="small" title="粗体" @click="wrap('**', '**')"><b>B</b></el-button>
        <el-button size="small" title="斜体" @click="wrap('*', '*')"><i>I</i></el-button>
        <el-button size="small" title="无序列表" @click="wrapLine('- ')">•</el-button>
        <el-button size="small" title="有序列表" @click="wrapLine('1. ')">1.</el-button>
        <el-button size="small" title="代码块" @click="wrap('```\n', '\n```')">&lt;/&gt;</el-button>
        <el-button size="small" title="链接" @click="wrap('[', '](url)')">🔗</el-button>
      </el-button-group>
      <span class="md-tip">markdown · 左侧编辑 / 右侧预览</span>
    </div>
    <div class="md-body">
      <textarea
        ref="taRef" class="md-input" :value="modelValue"
        :placeholder="placeholder" :rows="rows"
        @input="onInput"
      ></textarea>
      <div class="md-preview" v-html="html"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { marked } from 'marked'

const props = defineProps<{
  modelValue: string
  placeholder?: string
  rows?: number
}>()
const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

const taRef = ref<HTMLTextAreaElement>()
const html = computed(() => {
  try { return marked.parse(props.modelValue || '') as string } catch { return '' }
})

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
}

// 在选区两侧包裹前后缀
function wrap(before: string, after: string) {
  const ta = taRef.value
  if (!ta) return
  const { selectionStart: s, selectionEnd: e, value } = ta
  const selected = value.slice(s, e) || '文本'
  const next = value.slice(0, s) + before + selected + after + value.slice(e)
  emit('update:modelValue', next)
  requestAnimationFrame(() => {
    ta.focus()
    ta.selectionStart = s + before.length
    ta.selectionEnd = s + before.length + selected.length
  })
}

// 在行首加前缀（标题/列表）
function wrapLine(prefix: string) {
  const ta = taRef.value
  if (!ta) return
  const { selectionStart: s, value } = ta
  // 定位当前行起始
  const lineStart = value.lastIndexOf('\n', s - 1) + 1
  const next = value.slice(0, lineStart) + prefix + value.slice(lineStart)
  emit('update:modelValue', next)
  requestAnimationFrame(() => {
    ta.focus()
    const pos = s + prefix.length
    ta.selectionStart = ta.selectionEnd = pos
  })
}
</script>

<style scoped>
.md-editor { width: 100%; }
.md-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
.md-tip { color: #c0c4cc; font-size: 12px; }
.md-body { display: flex; gap: 12px; }
.md-input {
  flex: 1; font-family: Consolas, monospace; font-size: 13px; line-height: 1.7;
  border: 1px solid #dcdfe6; border-radius: 4px; padding: 8px 12px; resize: vertical;
  outline: none; transition: border-color 0.2s;
}
.md-input:focus { border-color: #409eff; }
.md-preview {
  flex: 1; border: 1px solid #ebeef5; border-radius: 4px; padding: 8px 12px;
  min-height: 200px; max-height: 320px; overflow-y: auto;
  font-size: 13px; line-height: 1.7; color: #606266; background: #fafafa;
}
.md-preview :deep(p) { margin: 4px 0; }
.md-preview :deep(h1), .md-preview :deep(h2), .md-preview :deep(h3) { margin: 10px 0 4px; }
.md-preview :deep(ul), .md-preview :deep(ol) { padding-left: 20px; }
.md-preview :deep(code) { background: #f0f2f5; padding: 1px 4px; border-radius: 3px; }
.md-preview :deep(pre) { background: #f0f2f5; padding: 8px; border-radius: 4px; overflow-x: auto; }
.md-preview :deep(pre code) { background: none; padding: 0; }
</style>
