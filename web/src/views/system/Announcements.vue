<template>
  <el-card shadow="never">
    <div class="toolbar">
      <span class="desc">公告发布后，控制台所有登录账号在顶部铃铛与概览页可见；内容支持 markdown</span>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>&nbsp;新建公告
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="title" label="标题" min-width="200">
        <template #default="{ row }">
          {{ row.title }}
          <el-tag v-if="row.level === 'important'" size="small" type="danger" style="margin-left: 6px">重要</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'published' ? 'success' : 'info'" size="small">
            {{ row.status === 'published' ? '已发布' : '草稿' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="publisher" label="发布者" width="120" />
      <el-table-column label="发布时间" width="170">
        <template #default="{ row }">{{ fmtDateTime(row.publishAt) }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ fmtDateTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="280" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openDetail(row)">查看</el-button>
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button
            v-if="row.status === 'draft'"
            link type="success" size="small" @click="togglePublish(row, true)">发布
          </el-button>
          <el-button
            v-else
            link type="warning" size="small" @click="togglePublish(row, false)">下线
          </el-button>
          <el-popconfirm title="确定删除该公告？" @confirm="del(row)">
            <template #reference><el-button link type="danger" size="small">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新建/编辑公告 -->
    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑公告' : '新建公告'" width="720px" :close-on-click-modal="false">
      <el-form label-width="70px" @submit.prevent>
        <el-form-item label="标题" required>
          <el-input v-model="editForm.title" maxlength="128" placeholder="公告标题" />
        </el-form-item>
        <el-form-item label="级别">
          <el-radio-group v-model="editForm.level">
            <el-radio value="normal">普通</el-radio>
            <el-radio value="important">重要</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="内容">
          <Md-editor v-model="editForm.content" placeholder="支持 markdown 语法" :rows="10" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button :loading="saving" @click="doSave('draft')">存草稿</el-button>
        <el-button type="primary" :loading="saving" @click="doSave('published')">保存并发布</el-button>
      </template>
    </el-dialog>

    <!-- 公告详情抽屉 -->
    <el-drawer v-model="detailVisible" title="公告详情" size="560px">
      <template v-if="detailRow">
        <div class="detail-head">
          <span class="detail-title">{{ detailRow.title }}</span>
          <el-tag v-if="detailRow.level === 'important'" size="small" type="danger">重要</el-tag>
          <el-tag :type="detailRow.status === 'published' ? 'success' : 'info'" size="small">
            {{ detailRow.status === 'published' ? '已发布' : '草稿' }}
          </el-tag>
        </div>
        <div class="detail-meta">
          发布者：{{ detailRow.publisher || '-' }} · 发布时间：{{ fmtDateTime(detailRow.publishAt) }} · 创建：{{ fmtDateTime(detailRow.createdAt) }}
        </div>
        <el-divider />
        <div class="detail-content" v-html="renderMd(detailRow.content)"></div>
      </template>
    </el-drawer>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { marked } from 'marked'
import { api, type Announcement } from '../../api'
import { fmtDateTime } from '../../utils/format'
import MdEditor from '../../components/MdEditor.vue'

const list = ref<Announcement[]>([])
const loading = ref(false)
const editVisible = ref(false)
const saving = ref(false)
const editForm = reactive({ id: 0, title: '', level: 'normal', content: '' })

// 公告详情
const detailVisible = ref(false)
const detailRow = ref<Announcement | null>(null)
const renderMd = (text: string) => { try { return marked.parse(text || '') as string } catch { return '' } }
function openDetail(row: Announcement) {
  detailRow.value = row
  detailVisible.value = true
}

async function load() {
  loading.value = true
  try {
    list.value = (await api.admin.announcements.list()) || []
  } catch { /* 错误已由拦截器提示 */ } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(editForm, { id: 0, title: '', level: 'normal', content: '' })
  editVisible.value = true
}

function openEdit(row: Announcement) {
  Object.assign(editForm, { id: row.id, title: row.title, level: row.level, content: row.content })
  editVisible.value = true
}

async function doSave(status: string) {
  if (!editForm.title.trim()) {
    ElMessage.warning('请输入标题')
    return
  }
  saving.value = true
  try {
    const data = { title: editForm.title, level: editForm.level, content: editForm.content, status }
    if (editForm.id) {
      await api.admin.announcements.update(editForm.id, data)
    } else {
      await api.admin.announcements.create(data)
    }
    ElMessage.success(status === 'published' ? '已发布' : '已保存')
    editVisible.value = false
    load()
  } catch { /* 错误已由拦截器提示 */ } finally {
    saving.value = false
  }
}

async function togglePublish(row: Announcement, publish: boolean) {
  try {
    await api.admin.announcements.update(row.id, {
      title: row.title, level: row.level, content: row.content, status: publish ? 'published' : 'draft',
    })
    ElMessage.success(publish ? '已发布' : '已下线')
    load()
  } catch { /* 错误已由拦截器提示 */ }
}

async function del(row: Announcement) {
  try {
    await api.admin.announcements.remove(row.id)
    ElMessage.success('已删除')
    load()
  } catch { /* 错误已由拦截器提示 */ }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.desc { color: #909399; font-size: 13px; }
.detail-head { display: flex; align-items: center; gap: 8px; }
.detail-title { font-size: 18px; font-weight: 600; color: #303133; }
.detail-meta { color: #909399; font-size: 12px; margin-top: 8px; }
.detail-content { color: #303133; font-size: 14px; line-height: 1.8; }
.detail-content :deep(p) { margin: 8px 0; }
.detail-content :deep(h1), .detail-content :deep(h2), .detail-content :deep(h3) { margin: 16px 0 8px; }
.detail-content :deep(ul), .detail-content :deep(ol) { padding-left: 24px; }
.detail-content :deep(code) { background: #f0f2f5; padding: 2px 6px; border-radius: 3px; font-family: Consolas, monospace; }
.detail-content :deep(pre) { background: #f6f8fa; padding: 12px; border-radius: 4px; overflow-x: auto; }
</style>
