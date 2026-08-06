<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-alert type="info" :closable="false" style="flex: 1; margin-right: 16px">
        OpenAPI 签名：X-App-Key + X-Timestamp(秒) + X-Sign = HMAC-SHA256(AppSecret, AppKey+Timestamp)，接口前缀 /openapi/v1
      </el-alert>
      <el-button type="primary" @click="dialogVisible = true">
        <el-icon><Plus /></el-icon>&nbsp;创建应用
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="name" label="应用名称" min-width="130" />
      <el-table-column label="AppKey" min-width="200">
        <template #default="{ row }">
          <el-text type="info">{{ row.appKey }}</el-text>
          <el-button link type="primary" size="small" @click="copy(row.appKey)">复制</el-button>
        </template>
      </el-table-column>
      <el-table-column label="AppSecret" min-width="240">
        <template #default="{ row }">
          <el-text type="info">{{ secretShown[row.id] ? row.appSecret : '********' }}</el-text>
          <el-button link type="primary" size="small" @click="secretShown[row.id] = !secretShown[row.id]">
            {{ secretShown[row.id] ? '隐藏' : '查看' }}
          </el-button>
          <el-button link type="primary" size="small" @click="copy(row.appSecret)">复制</el-button>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="80">
        <template #default="{ row }">
          <el-switch v-model="row.enabled" @change="toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right">
        <template #default="{ row }">
          <el-popconfirm title="确定删除该应用？" @confirm="del(row)">
            <template #reference><el-button link type="danger">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
    <div style="margin-top: 12px; display: flex; justify-content: flex-end">
      <el-pagination
        v-model:current-page="page"
        :page-size="size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>
  </el-card>

  <el-dialog v-model="dialogVisible" title="创建应用" width="420px">
    <el-form label-width="90px">
      <el-form-item label="应用名称" required>
        <el-input v-model="appName" placeholder="如：数据大屏应用" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type OpenApp } from '../api'
import { fmtDateTime } from '../utils/format'

const list = ref<OpenApp[]>([])
const loading = ref(false)
const page = ref(1)
const total = ref(0)
const size = 10
const dialogVisible = ref(false)
const saving = ref(false)
const appName = ref('')
const secretShown = reactive<Record<number, boolean>>({})

async function load() {
  loading.value = true
  try {
    const res = await api.listApps({ page: page.value, size })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!appName.value) {
    ElMessage.warning('请输入应用名称')
    return
  }
  saving.value = true
  try {
    await api.createApp(appName.value)
    ElMessage.success('应用已创建')
    dialogVisible.value = false
    appName.value = ''
    load()
  } finally {
    saving.value = false
  }
}

async function toggle(row: OpenApp) {
  await api.updateApp(row.id, { enabled: row.enabled })
  ElMessage.success(row.enabled ? '已启用' : '已禁用')
}

async function del(row: OpenApp) {
  await api.deleteApp(row.id)
  ElMessage.success('已删除')
  load()
}

function copy(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

function fmt(s: string) {
  return fmtDateTime(s)
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; align-items: center; margin-bottom: 16px; }
</style>
