<template>
  <el-card shadow="never">
    <template #header><span>消息轨迹查询</span></template>

    <!-- 搜索栏 -->
    <div class="toolbar">
      <div class="filters">
        <el-input v-model="search.traceId" placeholder="TraceID 精确搜索" clearable style="width: 240px" @change="load">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="search.productId" placeholder="全部产品" clearable style="width: 180px" @change="onProductChange">
          <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
        </el-select>
        <el-select v-model="search.deviceId" placeholder="全部设备" clearable style="width: 180px" @change="load">
          <el-option v-for="d in devices" :key="d.id" :label="d.name" :value="d.id" />
        </el-select>
        <el-select v-model="search.status" placeholder="全部状态" clearable style="width: 140px" @change="load">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="failed" />
          <el-option label="进行中" value="pending" />
        </el-select>
        <el-date-picker
          v-model="search.timeRange"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          style="width: 360px"
          @change="load"
        />
      </div>
      <el-button type="primary" @click="load">
        <el-icon><Search /></el-icon>&nbsp;查询
      </el-button>
    </div>

    <!-- 列表 -->
    <el-table :data="traces" v-loading="loading" stripe @row-click="showDetail">
      <el-table-column prop="traceId" label="TraceID" min-width="200">
        <template #default="{ row }">
          <el-text class="mono" size="small">{{ row.traceId }}</el-text>
        </template>
      </el-table-column>
      <el-table-column label="设备" min-width="120">
        <template #default="{ row }">{{ row.deviceName || '-' }}</template>
      </el-table-column>
      <el-table-column label="方向" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="row.direction === 'up' ? 'success' : 'warning'">
            {{ row.direction === 'up' ? '上行' : '下行' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="阶段" width="100">
        <template #default="{ row }">{{ row.stage || '-' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag size="small" :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="耗时" width="90">
        <template #default="{ row }">{{ calcDuration(row) }}</template>
      </el-table-column>
      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager" background layout="total, prev, pager, next"
      :total="total" :page-size="size" v-model:current-page="page" @current-change="load"
    />

    <!-- 详情抽屉 -->
    <el-drawer v-model="drawerVisible" title="轨迹详情" size="500px">
      <template v-if="currentTrace">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="TraceID">{{ currentTrace.traceId }}</el-descriptions-item>
          <el-descriptions-item label="设备">{{ currentTrace.deviceName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="方向">{{ currentTrace.direction === 'up' ? '上行' : '下行' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag size="small" :type="statusType(currentTrace.status)">{{ statusText(currentTrace.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="总耗时">{{ calcDuration(currentTrace) }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ fmt(currentTrace.createdAt) }}</el-descriptions-item>
        </el-descriptions>

        <h4 style="margin: 20px 0 12px">处理阶段</h4>
        <el-descriptions :column="1" border>
          <el-descriptions-item label="当前阶段">{{ currentTrace.stage || '-' }}</el-descriptions-item>
          <el-descriptions-item v-if="currentTrace.ingestMs != null" label="接入耗时">{{ currentTrace.ingestMs }}ms</el-descriptions-item>
          <el-descriptions-item v-if="currentTrace.decodeMs != null" label="解码耗时">{{ currentTrace.decodeMs }}ms</el-descriptions-item>
          <el-descriptions-item v-if="currentTrace.storeMs != null" label="存储耗时">{{ currentTrace.storeMs }}ms</el-descriptions-item>
          <el-descriptions-item v-if="currentTrace.ruleMs != null" label="规则耗时">{{ currentTrace.ruleMs }}ms</el-descriptions-item>
          <el-descriptions-item v-if="currentTrace.error" label="错误信息">
            <el-text type="danger">{{ currentTrace.error }}</el-text>
          </el-descriptions-item>
        </el-descriptions>
      </template>
    </el-drawer>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api, type Product, type Device } from '../api'
import { fmtDateTime } from '../utils/format'

const products = ref<Product[]>([])
const devices = ref<Device[]>([])
const traces = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const size = 20
const loading = ref(false)

const search = reactive({
  traceId: '',
  productId: null as number | null,
  deviceId: null as number | null,
  status: '',
  timeRange: null as [Date, Date] | null
})

const drawerVisible = ref(false)
const currentTrace = ref<any>(null)

async function loadProducts() {
  const res = await api.listProducts({ page: 1, size: 100 })
  products.value = res.list
}

async function onProductChange() {
  search.deviceId = null
  devices.value = []
  if (!search.productId) {
    load()
    return
  }
  const res = await api.listDevices({ productId: search.productId, page: 1, size: 200 })
  devices.value = res.list
  load()
}

async function load() {
  loading.value = true
  try {
    const params: any = { page: page.value, size }
    if (search.traceId) params.traceId = search.traceId
    if (search.deviceId) params.deviceId = search.deviceId
    if (search.status) params.status = search.status
    if (search.productId) {
      const p = products.value.find(x => x.id === search.productId)
      if (p) params.productId = p.productId
    }
    if (search.timeRange) {
      params.startTime = search.timeRange[0].toISOString()
      params.endTime = search.timeRange[1].toISOString()
    }
    const res = await api.traces.list(params) as any
    traces.value = res.list || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

async function showDetail(row: any) {
  try {
    const detail = await api.traces.get(row.traceId) as any
    currentTrace.value = detail
    drawerVisible.value = true
  } catch {
    currentTrace.value = row
    drawerVisible.value = true
  }
}

function statusType(s: string) {
  return ({ success: 'success', failed: 'danger', pending: 'warning' } as any)[s] || 'info'
}
function statusText(s: string) {
  return ({ success: '成功', failed: '失败', pending: '进行中' } as any)[s] || s
}
function calcDuration(row: any): string {
  const parts = [row.ingestMs, row.decodeMs, row.storeMs, row.ruleMs].filter((v: any) => v != null && v > 0)
  if (!parts.length) return '-'
  return parts.reduce((a: number, b: number) => a + b, 0) + 'ms'
}
function fmt(s: string | null) {
  return fmtDateTime(s)
}

onMounted(() => {
  loadProducts()
  load()
})
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.filters { display: flex; gap: 12px; flex-wrap: wrap; }
.pager { margin-top: 16px; justify-content: flex-end; }
.mono { font-family: monospace; }
</style>
