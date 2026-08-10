<template>
  <el-card shadow="never">
    <template #header><span>全局日志检索（平台超管：可跨账号检索任意设备的日志与消息轨迹）</span></template>

    <el-tabs v-model="tab">
      <!-- 设备日志 -->
      <el-tab-pane label="设备日志" name="logs">
        <div class="toolbar">
          <div class="filters">
            <el-select
              v-model="logSearch.deviceId" filterable remote clearable
              :remote-method="searchDevice" placeholder="按设备名搜索选择" style="width: 220px"
            >
              <el-option v-for="d in deviceOptions" :key="d.id" :label="d.name" :value="d.id" />
            </el-select>
            <el-select v-model="logSearch.category" placeholder="分类" clearable style="width: 140px" @change="loadLogs">
              <el-option label="数据上行" value="data_up" />
              <el-option label="指令下发" value="command" />
              <el-option label="告警" value="alarm" />
              <el-option label="事件" value="event" />
            </el-select>
            <el-input v-model="logSearch.keyword" placeholder="内容关键词（payload 模糊）" clearable style="width: 220px" @keyup.enter="loadLogs" />
            <el-button type="primary" @click="loadLogs">查询</el-button>
          </div>
        </div>

        <el-table :data="logs" v-loading="logLoading" stripe>
          <el-table-column prop="deviceName" label="设备" min-width="130" />
          <el-table-column label="分类" width="100">
            <template #default="{ row }">{{ categoryText(row.category) }}</template>
          </el-table-column>
          <el-table-column prop="summary" label="摘要" min-width="220" show-overflow-tooltip />
          <el-table-column label="时间" width="170">
            <template #default="{ row }">{{ fmtDateTime(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="80">
            <template #default="{ row }">
              <el-button link type="primary" size="small" @click="showLogPayload(row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          class="pager" background layout="total, prev, pager, next"
          :total="logTotal" :page-size="logSize" v-model:current-page="logPage" @current-change="loadLogs"
        />
      </el-tab-pane>

      <!-- 消息轨迹 -->
      <el-tab-pane label="消息轨迹" name="traces">
        <div class="toolbar">
          <div class="filters">
            <el-input v-model="traceSearch.traceId" placeholder="TraceID 精确搜索" clearable style="width: 240px" @change="loadTraces" />
            <el-select
              v-model="traceSearch.deviceId" filterable remote clearable
              :remote-method="searchDevice" placeholder="按设备名搜索选择" style="width: 220px"
            >
              <el-option v-for="d in deviceOptions" :key="d.id" :label="d.name" :value="d.id" />
            </el-select>
            <el-select v-model="traceSearch.status" placeholder="全部状态" clearable style="width: 140px" @change="loadTraces">
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
              <el-option label="进行中" value="pending" />
            </el-select>
            <el-date-picker
              v-model="traceSearch.timeRange" type="datetimerange" range-separator="至"
              start-placeholder="开始时间" end-placeholder="结束时间" style="width: 360px" @change="loadTraces"
            />
            <el-button type="primary" @click="loadTraces">查询</el-button>
          </div>
        </div>

        <el-table :data="traces" v-loading="traceLoading" stripe @row-click="showTraceDetail">
          <el-table-column prop="traceId" label="TraceID" min-width="200">
            <template #default="{ row }"><el-text class="mono" size="small">{{ row.traceId }}</el-text></template>
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
              <el-tag size="small" :type="traceStatusType(row.status)">{{ traceStatusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时间" width="170">
            <template #default="{ row }">{{ fmtDateTime(row.createdAt) }}</template>
          </el-table-column>
        </el-table>
        <el-pagination
          class="pager" background layout="total, prev, pager, next"
          :total="traceTotal" :page-size="traceSize" v-model:current-page="tracePage" @current-change="loadTraces"
        />
      </el-tab-pane>
    </el-tabs>

    <!-- 设备日志详情 -->
    <el-dialog v-model="logDrawer" title="日志详情" width="620px">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="设备">{{ currentLog.deviceName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="分类">{{ categoryText(currentLog.category) }}</el-descriptions-item>
        <el-descriptions-item label="时间">{{ fmtDateTime(currentLog.createdAt) }}</el-descriptions-item>
        <el-descriptions-item label="TraceID">{{ currentLog.traceId || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div class="payload" v-if="currentLog.payload">{{ currentLog.payload }}</div>
    </el-dialog>

    <!-- 轨迹详情 -->
    <el-drawer v-model="traceDrawer" title="轨迹详情" size="500px">
      <template v-if="currentTrace">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="TraceID">{{ currentTrace.traceId }}</el-descriptions-item>
          <el-descriptions-item label="设备">{{ currentTrace.deviceName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="方向">{{ currentTrace.direction === 'up' ? '上行' : '下行' }}</el-descriptions-item>
          <el-descriptions-item label="主题">{{ currentTrace.topic || '-' }}</el-descriptions-item>
          <el-descriptions-item label="阶段">{{ currentTrace.stage || '-' }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag size="small" :type="traceStatusType(currentTrace.status)">{{ traceStatusText(currentTrace.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item v-if="currentTrace.error" label="错误">{{ currentTrace.error }}</el-descriptions-item>
          <el-descriptions-item label="时间">{{ fmtDateTime(currentTrace.createdAt) }}</el-descriptions-item>
        </el-descriptions>
        <div class="payload" v-if="currentTrace.payload">{{ currentTrace.payload }}</div>
      </template>
    </el-drawer>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '../../api'
import { fmtDateTime } from '../../utils/format'

const tab = ref('logs')

// 设备远程搜索选项
const deviceOptions = ref<{ id: number; name: string }[]>([])
async function searchDevice(kw: string) {
  try {
    const data = await api.listDevices({ keyword: kw, page: 1, size: 20 })
    deviceOptions.value = (data.list || []).map((d: any) => ({ id: d.id, name: d.name }))
  } catch { /* ignore */ }
}

// ---- 设备日志 ----
const logs = ref<any[]>([])
const logLoading = ref(false)
const logPage = ref(1)
const logSize = 20
const logTotal = ref(0)
const logSearch = reactive({ deviceId: undefined as number | undefined, category: '', keyword: '' })
const logDrawer = ref(false)
const currentLog = ref<any>({})

const categoryMap: Record<string, string> = { data_up: '数据上行', command: '指令下发', alarm: '告警', event: '事件' }
const categoryText = (c: string) => categoryMap[c] || c || '-'

async function loadLogs() {
  logLoading.value = true
  try {
    const params: any = { page: logPage.value, size: logSize.value }
    if (logSearch.deviceId) params.deviceId = logSearch.deviceId
    if (logSearch.category) params.category = logSearch.category
    if (logSearch.keyword) params.keyword = logSearch.keyword
    const data = await api.deviceLogs.listAll(params)
    logs.value = data.list || []
    logTotal.value = data.total || 0
  } catch { /* 错误已由拦截器提示 */ } finally {
    logLoading.value = false
  }
}

function showLogPayload(row: any) {
  currentLog.value = row
  logDrawer.value = true
}

// ---- 消息轨迹 ----
const traces = ref<any[]>([])
const traceLoading = ref(false)
const tracePage = ref(1)
const traceSize = 20
const traceTotal = ref(0)
const traceSearch = reactive({
  traceId: '', deviceId: undefined as number | undefined, status: '', timeRange: null as [Date, Date] | null,
})
const traceDrawer = ref(false)
const currentTrace = ref<any>({})

function traceStatusType(s: string) {
  return s === 'success' ? 'success' : s === 'failed' ? 'danger' : 'warning'
}
function traceStatusText(s: string) {
  return s === 'success' ? '成功' : s === 'failed' ? '失败' : '进行中'
}

async function loadTraces() {
  traceLoading.value = true
  try {
    const params: any = { page: tracePage.value, size: traceSize.value }
    if (traceSearch.traceId) params.traceId = traceSearch.traceId
    if (traceSearch.deviceId) params.deviceId = traceSearch.deviceId
    if (traceSearch.status) params.status = traceSearch.status
    if (traceSearch.timeRange?.length === 2) {
      params.startTime = new Date(traceSearch.timeRange[0]).toISOString()
      params.endTime = new Date(traceSearch.timeRange[1]).toISOString()
    }
    const data = await api.traces.list(params)
    traces.value = data.list || []
    traceTotal.value = data.total || 0
  } catch { /* 错误已由拦截器提示 */ } finally {
    traceLoading.value = false
  }
}

async function showTraceDetail(row: any) {
  try {
    currentTrace.value = await api.traces.get(row.traceId)
    traceDrawer.value = true
  } catch { /* 错误已由拦截器提示 */ }
}

onMounted(() => {
  searchDevice('')
  loadLogs()
})
</script>

<style scoped>
.toolbar { margin-bottom: 12px; }
.filters { display: flex; gap: 10px; flex-wrap: wrap; }
.pager { margin-top: 12px; display: flex; justify-content: flex-end; }
.mono { font-family: Consolas, monospace; }
.payload {
  margin-top: 12px; background: #f6f8fa; border: 1px solid #ebeef5; border-radius: 4px;
  padding: 10px 12px; font-family: Consolas, monospace; font-size: 12px;
  white-space: pre-wrap; word-break: break-all; max-height: 300px; overflow-y: auto;
}
</style>
