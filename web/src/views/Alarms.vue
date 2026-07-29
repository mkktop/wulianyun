<template>
  <el-card shadow="never">
    <div class="toolbar">
      <div class="filters">
        <el-select v-model="status" placeholder="全部状态" clearable style="width: 140px" @change="load">
          <el-option label="告警中" value="firing" />
          <el-option label="已解决" value="resolved" />
        </el-select>
        <el-select v-model="level" placeholder="全部级别" clearable style="width: 140px" @change="load">
          <el-option label="警告" value="warning" />
          <el-option label="严重" value="critical" />
        </el-select>
      </div>
      <div class="stats">
        <el-tag type="danger" v-if="stats.firing">告警中 {{ stats.firing }}</el-tag>
        <el-tag type="info">今日 {{ stats.today }}</el-tag>
      </div>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column label="级别" width="90">
        <template #default="{ row }">
          <el-tag :type="row.level === 'critical' ? 'danger' : 'warning'" size="small">
            {{ row.level === 'critical' ? '严重' : '警告' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="ruleName" label="规则" min-width="120" />
      <el-table-column label="设备" min-width="120">
        <template #default="{ row }">
          <el-link type="primary" @click="$router.push(`/devices/${row.deviceId}`)">{{ row.deviceName }}</el-link>
        </template>
      </el-table-column>
      <el-table-column prop="message" label="告警内容" min-width="260" show-overflow-tooltip />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'firing' ? 'danger' : 'success'" size="small">
            {{ row.status === 'firing' ? '告警中' : '已解决' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="触发时间" width="170">
        <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.status === 'firing'" link type="primary" @click="resolve(row)">标记解决</el-button>
          <el-text v-else type="info" size="small">{{ fmt(row.resolvedAt) }}</el-text>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager" background layout="total, prev, pager, next"
      :total="total" :page-size="size" v-model:current-page="page" @current-change="load"
    />
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type Alarm } from '../api'
import { realtime } from '../utils/realtime'

const list = ref<Alarm[]>([])
const total = ref(0)
const page = ref(1)
const size = 15
const status = ref('')
const level = ref('')
const loading = ref(false)
const stats = ref({ firing: 0, today: 0 })

async function load() {
  loading.value = true
  try {
    const res = await api.listAlarms({ page: page.value, size, status: status.value, level: level.value })
    list.value = res.list
    total.value = res.total
    stats.value = await api.alarmStats()
  } finally {
    loading.value = false
  }
}

async function resolve(row: Alarm) {
  await api.resolveAlarm(row.id)
  ElMessage.success('已标记解决')
  load()
}

function fmt(s: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
}

// 新告警实时刷新列表
function onMsg(msg: any) {
  if (msg.type === 'alarm') load()
}

onMounted(() => {
  load()
  realtime.on(onMsg)
})
onUnmounted(() => realtime.off(onMsg))
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.filters { display: flex; gap: 12px; }
.stats { display: flex; gap: 8px; align-items: center; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
