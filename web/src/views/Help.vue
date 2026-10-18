<template>
  <el-card shadow="never" class="help-card">
    <div class="help-layout">
      <!-- 左侧文档目录 -->
      <aside class="help-nav">
        <div class="help-nav-title">帮助中心</div>
        <el-menu :default-active="activeKey" @select="onSelect">
          <el-menu-item v-for="d in docs" :key="d.key" :index="d.key">
            <span>{{ d.title }}</span>
          </el-menu-item>
        </el-menu>
        <div class="help-nav-footer">
          没找到答案？查看
          <a href="/developer/" target="_blank" rel="noopener">完整开发文档</a>
        </div>
      </aside>

      <!-- 右侧内容 -->
      <div class="help-body" v-loading="loading">
        <template v-if="current">
          <h2 class="help-title">{{ current.title }}</h2>
          <div class="help-meta" v-if="current.updatedAt">更新于 {{ fmt(current.updatedAt) }}</div>
          <div class="help-content" v-html="rendered"></div>
        </template>
        <el-empty v-else-if="!loading" description="暂无帮助文档" :image-size="100">
          <div class="empty-guide">
            <p class="guide-tip">平台使用问题可先查阅开发文档；管理员可在「系统管理 → 帮助中心」发布文档</p>
            <el-button type="primary" @click="openDocs">查看开发文档</el-button>
          </div>
        </el-empty>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { marked } from 'marked'
import { api } from '../api'
import { fmtDateTime } from '../utils/format'

interface HelpDoc {
  id: number
  key: string
  title: string
  content: string
  updatedAt: string
}

const docs = ref<{ id: number; key: string; title: string }[]>([])
const current = ref<HelpDoc | null>(null)
const activeKey = ref('')
const loading = ref(false)

// 文档内容由超管在系统管理中维护（可信内容），marked 渲染
const rendered = computed(() => (current.value?.content ? (marked.parse(current.value.content) as string) : ''))

function onSelect(key: string) {
  loadDoc(key)
}

async function loadDoc(key: string) {
  loading.value = true
  try {
    current.value = await api.helpDocs.get(key)
    activeKey.value = key
  } finally {
    loading.value = false
  }
}

function openDocs() {
  window.open('/developer/', '_blank', 'noopener')
}

function fmt(s: string) {
  return fmtDateTime(s)
}

onMounted(async () => {
  try {
    const list = (await api.helpDocs.list()) || []
    docs.value = list
    if (list.length) loadDoc(list[0].key)
  } catch {
    /* 列表失败时展示空态引导 */
  }
})
</script>

<style scoped>
.help-card { min-height: calc(100vh - 140px); }
.help-layout { display: flex; gap: 20px; min-height: 480px; }
.help-nav { width: 220px; flex-shrink: 0; border-right: 1px solid #ebeef5; }
.help-nav-title { font-weight: 600; font-size: 15px; padding: 4px 20px 12px; }
.help-nav :deep(.el-menu) { border-right: none; }
.help-nav :deep(.el-menu-item) { height: 42px; line-height: 42px; }
.help-nav :deep(.el-menu-item.is-active) { background: #ecf5ff; border-radius: 4px; }
.help-nav-footer {
  padding: 14px 20px; color: #909399; font-size: 12px; line-height: 1.8;
  border-top: 1px solid #f0f0f0; margin-top: 12px;
}
.help-nav-footer a { color: #409eff; text-decoration: none; }
.help-body { flex: 1; min-width: 0; padding: 4px 8px; }
.help-title { margin: 0 0 4px; font-size: 20px; color: #303133; }
.help-meta { color: #c0c4cc; font-size: 12px; margin-bottom: 14px; }
.help-content { color: #303133; font-size: 14px; line-height: 1.8; word-break: break-word; }
.help-content :deep(h1), .help-content :deep(h2), .help-content :deep(h3) { margin: 18px 0 8px; color: #303133; }
.help-content :deep(p) { margin: 8px 0; }
.help-content :deep(ul), .help-content :deep(ol) { padding-left: 22px; }
.help-content :deep(code) {
  background: #f0f2f5; padding: 1px 5px; border-radius: 3px;
  font-family: Consolas, 'Courier New', monospace; font-size: 13px;
}
.help-content :deep(pre) {
  background: #0d1117; color: #e6edf3; padding: 14px; border-radius: 8px;
  overflow-x: auto; margin: 10px 0;
}
.help-content :deep(pre code) { background: none; color: inherit; padding: 0; }
.help-content :deep(blockquote) {
  margin: 10px 0; padding: 6px 14px; border-left: 3px solid #409eff;
  background: #ecf5ff; color: #606266; border-radius: 0 4px 4px 0;
}
.help-content :deep(table) { border-collapse: collapse; margin: 10px 0; width: 100%; }
.help-content :deep(th), .help-content :deep(td) { border: 1px solid #ebeef5; padding: 8px 12px; font-size: 13px; text-align: left; }
.help-content :deep(th) { background: #f5f7fa; }
.help-content :deep(a) { color: #409eff; text-decoration: none; }
.help-content :deep(img) { max-width: 100%; border-radius: 6px; }
.empty-guide { display: flex; flex-direction: column; align-items: center; gap: 10px; }
.guide-tip { color: #909399; font-size: 13px; margin: 0; }
</style>
