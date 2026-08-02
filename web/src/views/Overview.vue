<template>
  <div>
    <el-row :gutter="16">
      <el-col :span="4" v-for="card in cards" :key="card.label">
        <el-card shadow="hover" class="stat-card" :body-style="{ padding: '0' }">
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

    <el-row :gutter="16" class="chart-row">
      <el-col :span="16">
        <el-card shadow="never" class="chart-card">
          <template #header>
            <div class="card-head"><span>近7日消息量趋势</span></div>
          </template>
          <div ref="chartRef" class="chart"></div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="never" class="chart-card">
          <template #header>
            <div class="card-head"><span>设备状态分布</span></div>
          </template>
          <div ref="pieRef" class="chart"></div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import * as echarts from 'echarts'
import { api } from '../api'
import { realtime } from '../utils/realtime'

const data = ref<any>({ productCount: 0, deviceCount: 0, onlineCount: 0, onlineRate: 0, msgToday: 0, msgTotal: 0, msgRateMin: 0, msgTrend: [], statusDist: [] })
const chartRef = ref<HTMLElement>()
const pieRef = ref<HTMLElement>()
const chart = shallowRef<echarts.ECharts>()
const pie = shallowRef<echarts.ECharts>()

const cards = computed(() => [
  { label: '产品总数', value: data.value.productCount, icon: 'Box', color: '#409EFF', bg: '#ecf5ff' },
  { label: '设备总数', value: data.value.deviceCount, icon: 'Cpu', color: '#67C23A', bg: '#f0f9eb' },
  { label: '在线设备', value: data.value.onlineCount, icon: 'Connection', color: '#E6A23C', bg: '#fdf6ec' },
  { label: '在线率', value: (data.value.onlineRate ?? 0) + '%', icon: 'DataLine', color: '#36CFC9', bg: '#e6fffb' },
  { label: '今日消息', value: data.value.msgToday, icon: 'ChatDotSquare', color: '#F56C6C', bg: '#fef0f0' },
  { label: '吞吐量/分', value: data.value.msgRateMin ?? 0, icon: 'Histogram', color: '#722ED1', bg: '#f9f0ff' }
])

const statusNames: Record<string, string> = { online: '在线', offline: '离线', inactive: '未激活', disabled: '已禁用' }
const statusColors: Record<string, string> = { online: '#67C23A', offline: '#909399', inactive: '#E6A23C', disabled: '#F56C6C' }

async function load() {
  data.value = await api.overview()
  renderChart()
  renderPie()
}

function renderChart() {
  if (!chartRef.value) return
  if (!chart.value) chart.value = echarts.init(chartRef.value)
  const trend: { day: string; count: number }[] = data.value.msgTrend || []
  chart.value.setOption({
    grid: { left: 50, right: 20, top: 20, bottom: 30 },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: trend.map((t) => t.day), axisTick: { show: false } },
    yAxis: { type: 'value', splitLine: { lineStyle: { type: 'dashed' } } },
    series: [{
      type: 'bar', name: '消息量', barMaxWidth: 36,
      itemStyle: {
        borderRadius: [4, 4, 0, 0],
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: '#66b1ff' }, { offset: 1, color: '#409EFF' }
        ])
      },
      data: trend.map((t) => t.count)
    }]
  })
}

function renderPie() {
  if (!pieRef.value) return
  if (!pie.value) pie.value = echarts.init(pieRef.value)
  const dist: { status: string; count: number }[] = data.value.statusDist || []
  const pieData = dist.map((s) => ({
    name: statusNames[s.status] || s.status,
    value: s.count,
    itemStyle: { color: statusColors[s.status] || '#909399' }
  }))
  pie.value.setOption({
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: { bottom: 0, icon: 'circle' },
    series: [{
      type: 'pie', radius: ['45%', '68%'], center: ['50%', '44%'],
      avoidLabelOverlap: true,
      label: { show: true, formatter: '{b}\n{c}' },
      data: pieData.length ? pieData : [{ name: '暂无设备', value: 1, itemStyle: { color: '#ebeef5' } }]
    }]
  })
}

// 设备状态变化时刷新统计
function onMsg(msg: any) {
  if (msg.type === 'device_status') load()
}

const onResize = () => { chart.value?.resize(); pie.value?.resize() }

onMounted(() => {
  load()
  realtime.on(onMsg)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  realtime.off(onMsg)
  window.removeEventListener('resize', onResize)
  chart.value?.dispose()
  pie.value?.dispose()
})
</script>

<style scoped>
.stat-card { border-radius: 10px; overflow: hidden; transition: transform 0.2s; }
.stat-card:hover { transform: translateY(-2px); }
.stat {
  display: flex; align-items: center; gap: 12px; padding: 16px;
  border-top: 3px solid transparent;
}
.stat-icon {
  width: 48px; height: 48px; border-radius: 10px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.stat-body { min-width: 0; }
.num { font-size: 24px; font-weight: 700; line-height: 1.2; color: #303133; }
.label { color: #909399; font-size: 13px; margin-top: 4px; }
.chart-row { margin-top: 16px; }
.card-head { font-weight: 600; }
.chart-card { border-radius: 10px; }
.chart { height: 340px; }
</style>
