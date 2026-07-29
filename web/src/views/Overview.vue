<template>
  <div>
    <el-row :gutter="16">
      <el-col :span="6" v-for="card in cards" :key="card.label">
        <el-card shadow="hover" class="stat-card">
          <div class="stat">
            <el-icon :size="40" :color="card.color"><component :is="card.icon" /></el-icon>
            <div>
              <div class="num">{{ card.value }}</div>
              <div class="label">{{ card.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="never" class="chart-card">
      <template #header>近7日消息量趋势</template>
      <div ref="chartRef" class="chart"></div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import * as echarts from 'echarts'
import { api } from '../api'
import { realtime } from '../utils/realtime'

const data = ref<any>({ productCount: 0, deviceCount: 0, onlineCount: 0, msgToday: 0, msgTrend: [] })
const chartRef = ref<HTMLElement>()
const chart = shallowRef<echarts.ECharts>()

const cards = computed(() => [
  { label: '产品总数', value: data.value.productCount, icon: 'Box', color: '#409EFF' },
  { label: '设备总数', value: data.value.deviceCount, icon: 'Cpu', color: '#67C23A' },
  { label: '在线设备', value: data.value.onlineCount, icon: 'Connection', color: '#E6A23C' },
  { label: '今日消息', value: data.value.msgToday, icon: 'ChatDotSquare', color: '#F56C6C' }
])

async function load() {
  data.value = await api.overview()
  renderChart()
}

function renderChart() {
  if (!chartRef.value) return
  if (!chart.value) chart.value = echarts.init(chartRef.value)
  const trend: { day: string; count: number }[] = data.value.msgTrend || []
  chart.value.setOption({
    grid: { left: 50, right: 20, top: 30, bottom: 30 },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: trend.map((t) => t.day) },
    yAxis: { type: 'value' },
    series: [{
      type: 'bar', name: '消息量', barMaxWidth: 40,
      itemStyle: { color: '#409EFF', borderRadius: [4, 4, 0, 0] },
      data: trend.map((t) => t.count)
    }]
  })
}

// 设备状态变化时刷新统计
function onMsg(msg: any) {
  if (msg.type === 'device_status') load()
}

const onResize = () => chart.value?.resize()

onMounted(() => {
  load()
  realtime.on(onMsg)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  realtime.off(onMsg)
  window.removeEventListener('resize', onResize)
  chart.value?.dispose()
})
</script>

<style scoped>
.stat-card .stat { display: flex; align-items: center; gap: 16px; }
.num { font-size: 26px; font-weight: 700; }
.label { color: #999; font-size: 13px; }
.chart-card { margin-top: 16px; }
.chart { height: 360px; }
</style>
