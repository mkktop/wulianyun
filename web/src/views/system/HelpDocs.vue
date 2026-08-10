<template>
  <el-card shadow="never">
    <div class="toolbar">
      <span class="desc">帮助中心文档（markdown 入库、前端渲染），控制台所有登录账号可在「帮助」中查看</span>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>&nbsp;新建文档
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="key" label="标识" min-width="160">
        <template #default="{ row }"><code class="key">{{ row.key }}</code></template>
      </el-table-column>
      <el-table-column prop="title" label="标题" min-width="200" />
      <el-table-column label="更新时间" width="170">
        <template #default="{ row }">{{ fmtDateTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-popconfirm title="确定删除该文档？" @confirm="del(row)">
            <template #reference><el-button link type="danger" size="small">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新建/编辑文档 -->
    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑文档' : '新建文档'" width="720px" :close-on-click-modal="false">
      <el-form label-width="70px" @submit.prevent>
        <el-form-item label="标识" required>
          <el-input v-model="editForm.key" :disabled="!!editForm.id" placeholder="英文标识（如 getting-started），用于前端访问路径" />
        </el-form-item>
        <el-form-item label="标题" required>
          <el-input v-model="editForm.title" maxlength="128" placeholder="文档标题" />
        </el-form-item>
        <el-form-item label="内容">
          <Md-editor v-model="editForm.content" placeholder="支持 markdown 语法" :rows="10" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doSave">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type HelpDocItem } from '../../api'
import { fmtDateTime } from '../../utils/format'
import MdEditor from '../../components/MdEditor.vue'

const list = ref<HelpDocItem[]>([])
const loading = ref(false)
const editVisible = ref(false)
const saving = ref(false)
const editForm = reactive({ id: 0, key: '', title: '', content: '' })

async function load() {
  loading.value = true
  try {
    list.value = (await api.admin.helpDocs.list()) || []
  } catch { /* 错误已由拦截器提示 */ } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(editForm, { id: 0, key: '', title: '', content: '' })
  editVisible.value = true
}

function openEdit(row: HelpDocItem) {
  Object.assign(editForm, { id: row.id, key: row.key, title: row.title, content: row.content })
  editVisible.value = true
}

async function doSave() {
  if (!editForm.key.trim() || !editForm.title.trim()) {
    ElMessage.warning('标识与标题必填')
    return
  }
  saving.value = true
  try {
    const data = { key: editForm.key, title: editForm.title, content: editForm.content }
    if (editForm.id) {
      await api.admin.helpDocs.update(editForm.id, data)
    } else {
      await api.admin.helpDocs.create(data)
    }
    ElMessage.success('已保存')
    editVisible.value = false
    load()
  } catch { /* 错误已由拦截器提示 */ } finally {
    saving.value = false
  }
}

async function del(row: HelpDocItem) {
  try {
    await api.admin.helpDocs.remove(row.id)
    ElMessage.success('已删除')
    load()
  } catch { /* 错误已由拦截器提示 */ }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.desc { color: #909399; font-size: 13px; }
.key { font-family: Consolas, monospace; color: #409eff; }
</style>
