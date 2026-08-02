<template>
  <div>
    <!-- 统计卡片 -->
    <el-row :gutter="16" class="stat-row">
      <el-col :span="6" v-for="card in statCards" :key="card.label">
        <el-card shadow="hover" class="stat-card" :body-style="{ padding: 0 }">
          <div class="stat" :style="{ borderTopColor: card.color }">
            <div class="stat-icon" :style="{ background: card.bg, color: card.color }">
              <el-icon :size="24"><component :is="card.icon" /></el-icon>
            </div>
            <div class="stat-body">
              <div class="num">{{ card.value }}</div>
              <div class="label">{{ card.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 告警趋势图 -->
    <el-card shadow="never" class="trend-card">
      <template #header><span style="font-weight:600">近7日告警趋势</span></template>
      <div ref="trendRef" class="trend-chart"></div>
    </el-card>

    <!-- 告警列表 -->
    <el-card shadow="never" style="margin-top: 16px">
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
        <el-table-column prop="message" label="告警内容" min-width="220" show-overflow-tooltip />
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
        <el-table-column label="恢复时间" width="170">
          <template #default="{ row }">
            <el-text v-if="row.status === 'resolved'" type="success" size="small">{{ fmt(row.resolvedAt) }}</el-text>
            <el-text v-else type="info" size="small">-</el-text>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'firing'">
              <el-button link type="primary" @click="confirm(row)" :disabled="!!row.confirmedAt">确认</el-button>
              <el-button link type="success" @click="resolve(row)">标记解决</el-button>
            </template>
            <template v-else>
              <el-text type="info" size="small">已恢复</el-text>
            </template>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        class="pager" background layout="total, prev, pager, next"
        :total="total" :page-size="size" v-model:current-page="page" @current-change="load"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { api, type Alarm } from '../api'
import { realtime } from '../utils/realtime'

const list = ref<Alarm[]>([])
const total = ref(0)
const page = ref(1)
const size = 15
const status = ref('')
const level = ref('')
const loading = ref(false)
const stats = ref({ total: 0, firing: 0, resolved: 0, today: 0 })

// 趋势图
const trendRef = ref<HTMLElement>()
const trendChart = shallowRef<echarts.ECharts>()

const statCards = computed(() => [
  { label: '总告警数', value: stats.value.total, icon: 'Bell', color: '#409EFF', bg: '#ecf5ff' },
  { label: '活跃告警', value: stats.value.firing, icon: 'WarningFilled', color: '#F56C6C', bg: '#fef0f0' },
  { label: '已恢复', value: stats.value.resolved, icon: 'CircleCheck', color: '#67C23A', bg: '#f0f9eb' },
  { label: '今日告警', value: stats.value.today, icon: 'Timer', color: '#E6A23C', bg: '#fdf6ec' },
])

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

async function loadTrend() {
  try {
    const data = await api.alarmTrend()
    renderTrend(data)
  } catch {
    // 后端可能未实现，静默处理
  }
}

function renderTrend(data: { day: string; count: number }[]) {
  if (!trendRef.value) return
  if (!trendChart.value) trendChart.value = echarts.init(trendRef.value)
  trendChart.value.setOption({
    grid: { left: 46, right: 20, top: 16, bottom: 28 },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: data.map(d => d.day), axisTick: { show: false } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { type: 'dashed' } } },
    series: [{
      type: 'line', smooth: true, showSymbol: false,
      areaStyle: { opacity: 0.12 },
      lineStyle: { width: 2 },
      itemStyle: { color: '#F56C6C' },
      data: data.map(d => d.count)
    }]
  })
}

async function resolve(row: Alarm) {
  await api.resolveAlarm(row.id)
  ElMessage.success('已标记解决')
  load()
}

async function confirm(row: Alarm) {
  await api.confirmAlarm(row.id)
  ElMessage.success('已确认')
  load()
}

function fmt(s: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
}

// 新告警实时刷新列表
function onMsg(msg: any) {
  if (msg.type === 'alarm') load()
}

const onResize = () => { trendChart.value?.resize() }

onMounted(() => {
  load()
  loadTrend()
  realtime.on(onMsg)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  realtime.off(onMsg)
  window.removeEventListener('resize', onResize)
  trendChart.value?.dispose()
})
</script>

<style scoped>
.stat-row { margin-bottom: 16px; }
.stat-card { border-radius: 10px; overflow: hidden; transition: transform 0.2s; }
.stat-card:hover { transform: translateY(-2px); }
.stat {
  display: flex; align-items: center; gap: 14px; padding: 18px;
  border-top: 3px solid transparent;
}
.stat-icon {
  width: 48px; height: 48px; border-radius: 10px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.stat-body { min-width: 0; }
.num { font-size: 26px; font-weight: 700; line-height: 1.2; color: #303133; }
.label { color: #909399; font-size: 13px; margin-top: 2px; }

.trend-card { border-radius: 10px; }
.trend-chart { height: 220px; }

.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.filters { display: flex; gap: 12px; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
