<template>
  <div class="screen">
    <div class="screen-header">
      <h1>物联云平台 · 数据大屏</h1>
      <div class="time">{{ now }}</div>
      <el-button link class="exit" @click="$router.push('/overview')">
        <el-icon color="#7ee7ff" :size="20"><Close /></el-icon>
      </el-button>
    </div>

    <div class="kpi-row">
      <div class="kpi" v-for="k in kpis" :key="k.label">
        <div class="kpi-value">{{ k.value }}</div>
        <div class="kpi-label">{{ k.label }}</div>
      </div>
    </div>

    <div class="grid">
      <div class="panel">
        <div class="panel-title">近7日消息量</div>
        <div ref="trendRef" class="chart"></div>
      </div>
      <div class="panel">
        <div class="panel-title">设备状态分布</div>
        <div ref="pieRef" class="chart"></div>
      </div>
      <div class="panel">
        <div class="panel-title">实时告警</div>
        <div class="alarm-list">
          <div v-for="a in alarms" :key="a.id" class="alarm-item" :class="a.level">
            <span class="alarm-time">{{ shortTime(a.createdAt) }}</span>
            <span class="alarm-msg">{{ a.message }}</span>
          </div>
          <div v-if="!alarms.length" class="alarm-empty">暂无告警</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import * as echarts from 'echarts'
import { api, type Alarm } from '../api'
import { realtime } from '../utils/realtime'

const data = ref<any>({ productCount: 0, deviceCount: 0, onlineCount: 0, msgToday: 0, msgTrend: [], statusDist: [] })
const alarms = ref<Alarm[]>([])
const now = ref('')
const trendRef = ref<HTMLElement>()
const pieRef = ref<HTMLElement>()
const trendChart = shallowRef<echarts.ECharts>()
const pieChart = shallowRef<echarts.ECharts>()
let timer = 0
let clockTimer = 0

const kpis = computed(() => [
  { label: '产品总数', value: data.value.productCount },
  { label: '设备总数', value: data.value.deviceCount },
  { label: '在线设备', value: data.value.onlineCount },
  {
    label: '在线率',
    value: data.value.deviceCount ? Math.round((data.value.onlineCount / data.value.deviceCount) * 100) + '%' : '-'
  },
  { label: '今日消息', value: data.value.msgToday }
])

const statusNames: Record<string, string> = { online: '在线', offline: '离线', inactive: '未激活', disabled: '已禁用' }
const statusColors: Record<string, string> = { online: '#3be8b0', offline: '#5b8ff9', inactive: '#f6bd16', disabled: '#e8684a' }

async function load() {
  data.value = await api.overview()
  const res = await api.listAlarms({ page: 1, size: 8 })
  alarms.value = res.list
  render()
}

function render() {
  if (trendRef.value) {
    if (!trendChart.value) trendChart.value = echarts.init(trendRef.value)
    const trend = data.value.msgTrend || []
    trendChart.value.setOption({
      grid: { left: 50, right: 16, top: 20, bottom: 26 },
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: trend.map((t: any) => t.day), axisLine: { lineStyle: { color: '#2a5674' } }, axisLabel: { color: '#7ee7ff' } },
      yAxis: { type: 'value', splitLine: { lineStyle: { color: '#12324a' } }, axisLabel: { color: '#7ee7ff' } },
      series: [{
        type: 'line', smooth: true, showSymbol: false,
        lineStyle: { color: '#00d4ff', width: 3 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(0,212,255,0.4)' }, { offset: 1, color: 'rgba(0,212,255,0)' }
          ])
        },
        data: trend.map((t: any) => t.count)
      }]
    })
  }
  if (pieRef.value) {
    if (!pieChart.value) pieChart.value = echarts.init(pieRef.value)
    const dist = (data.value.statusDist || []).map((s: any) => ({
      name: statusNames[s.status] || s.status,
      value: s.count,
      itemStyle: { color: statusColors[s.status] || '#888' }
    }))
    pieChart.value.setOption({
      tooltip: { trigger: 'item' },
      legend: { bottom: 0, textStyle: { color: '#7ee7ff' } },
      series: [{
        type: 'pie', radius: ['45%', '70%'], center: ['50%', '45%'],
        label: { color: '#cde', formatter: '{b}: {c}' },
        data: dist
      }]
    })
  }
}

function shortTime(s: string) {
  return new Date(s).toLocaleTimeString('zh-CN', { hour12: false })
}

function onMsg(msg: any) {
  if (msg.type === 'alarm' || msg.type === 'device_status') load()
}

const onResize = () => { trendChart.value?.resize(); pieChart.value?.resize() }

onMounted(() => {
  load()
  realtime.on(onMsg)
  timer = window.setInterval(load, 30000)
  clockTimer = window.setInterval(() => { now.value = new Date().toLocaleString('zh-CN', { hour12: false }) }, 1000)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  realtime.off(onMsg)
  window.clearInterval(timer)
  window.clearInterval(clockTimer)
  window.removeEventListener('resize', onResize)
  trendChart.value?.dispose()
  pieChart.value?.dispose()
})
</script>

<style scoped>
.screen {
  height: 100vh; overflow: hidden; padding: 0 24px 24px;
  background: radial-gradient(ellipse at top, #0b2239 0%, #061120 70%);
  display: flex; flex-direction: column;
}
.screen-header {
  position: relative; text-align: center; padding: 18px 0 8px;
}
.screen-header h1 {
  color: #7ee7ff; font-size: 28px; letter-spacing: 6px;
  text-shadow: 0 0 20px rgba(0, 212, 255, 0.6);
}
.time { color: #4d7ea8; margin-top: 4px; font-size: 13px; }
.exit { position: absolute; right: 0; top: 20px; }
.kpi-row { display: flex; gap: 16px; margin: 16px 0; }
.kpi {
  flex: 1; text-align: center; padding: 18px 0;
  background: rgba(0, 212, 255, 0.05); border: 1px solid #12466b; border-radius: 8px;
}
.kpi-value { color: #00d4ff; font-size: 34px; font-weight: 700; }
.kpi-label { color: #7ea8c8; margin-top: 6px; font-size: 13px; }
.grid { flex: 1; display: grid; grid-template-columns: 2fr 1fr 1fr; gap: 16px; min-height: 0; }
.panel {
  background: rgba(0, 212, 255, 0.04); border: 1px solid #12466b; border-radius: 8px;
  padding: 14px; display: flex; flex-direction: column; min-height: 0;
}
.panel-title { color: #7ee7ff; font-size: 15px; margin-bottom: 8px; }
.chart { flex: 1; min-height: 0; }
.alarm-list { flex: 1; overflow-y: auto; }
.alarm-item {
  display: flex; gap: 10px; padding: 8px 10px; margin-bottom: 6px; border-radius: 4px;
  background: rgba(246, 189, 22, 0.08); border-left: 3px solid #f6bd16;
  color: #d8e6f0; font-size: 13px;
}
.alarm-item.critical { background: rgba(232, 104, 74, 0.1); border-left-color: #e8684a; }
.alarm-time { color: #7ea8c8; flex-shrink: 0; }
.alarm-msg { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.alarm-empty { color: #4d7ea8; text-align: center; margin-top: 40px; }
</style>
