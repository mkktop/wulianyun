<template>
  <div v-loading="loading">
    <!-- 新手引导：产品/设备/数据任一未就绪时展示（可关闭，沉淀在 localStorage） -->
    <el-card v-if="showOnboarding" shadow="never" class="onboard-card">
      <div class="onboard-head">
        <span class="onboard-title"><el-icon color="#409EFF"><Compass /></el-icon>快速上手 · 四步接入第一台设备</span>
        <el-button link size="small" @click="dismissOnboarding">不再显示</el-button>
      </div>
      <div class="onboard-steps">
        <div v-for="(s, i) in onboardingSteps" :key="i" class="onboard-step" :class="{ done: s.done }">
          <span class="step-dot">{{ s.done ? '✓' : i + 1 }}</span>
          <div class="step-body">
            <div class="step-name">{{ s.name }}<el-tag v-if="s.done" size="small" type="success" effect="plain">已完成</el-tag></div>
            <div class="step-desc">{{ s.desc }}</div>
          </div>
          <el-button v-if="!s.done" size="small" :type="i === nextStep ? 'primary' : 'default'" @click="$router.push(s.to)">{{ s.action }}</el-button>
        </div>
      </div>
    </el-card>

    <!-- 平台公告（已发布） -->
    <el-card v-if="announcements.length" shadow="never" class="announce-card">
      <div class="announce-inner">
        <el-icon class="announce-icon" color="#E6A23C"><BellFilled /></el-icon>
        <div class="announce-body">
          <div v-for="a in announcements.slice(0, 3)" :key="a.id" class="announce-item">
            <span class="announce-title" :class="{ important: a.level === 'important' }">{{ a.title }}</span>
            <span class="announce-meta">{{ fmtDate(a.publishAt) }}</span>
          </div>
        </div>
      </div>
    </el-card>

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
import echarts from '../utils/echarts'
import { api, type Announcement } from '../api'
import { realtime } from '../utils/realtime'
import { debounce } from '../utils/debounce'
import { fmtDate } from '../utils/format'

const data = ref<any>({ productCount: 0, deviceCount: 0, onlineCount: 0, onlineRate: 0, msgToday: 0, msgTotal: 0, msgRateMin: 0, msgTrend: [], statusDist: [] })
const loading = ref(false)
const announcements = ref<Announcement[]>([])
const chartRef = ref<HTMLElement>()
const pieRef = ref<HTMLElement>()
const chart = shallowRef<echarts.ECharts>()
const pie = shallowRef<echarts.ECharts>()

// ---- 新手引导 ----
const onboardDismissed = ref(localStorage.getItem('onboardingDismissed') === '1')
// 三项均就绪（有产品、有设备、有数据）或用户关闭后不再显示
const showOnboarding = computed(() =>
  !onboardDismissed.value && !(data.value.productCount > 0 && data.value.deviceCount > 0 && (data.value.msgTotal > 0 || data.value.msgToday > 0)))
const onboardingSteps = computed(() => {
  const d = data.value
  const steps = [
    { name: '创建产品', desc: '选择接入协议（MQTT / TCP / HTTP）与数据模式，获得产品 ProductID', to: '/console/products/new', action: '去创建', done: d.productCount > 0 },
    { name: '添加设备', desc: '在产品下添加设备，获得三元组（ProductID / DeviceName / DeviceSecret）', to: '/console/devices', action: '去添加', done: d.deviceCount > 0 },
    { name: '设备连接上报', desc: '设备用三元组连接平台并上报数据，可先用设备模拟器验证链路', to: '/console/tools/simulator', action: '用模拟器试一试', done: d.msgTotal > 0 || d.msgToday > 0 },
    { name: '配置规则与告警', desc: '为关键指标设置阈值告警或数据转发，异常第一时间触达', to: '/console/rules', action: '去配置', done: false }
  ]
  return steps
})
// 第一个未完成步骤高亮为主按钮
const nextStep = computed(() => {
  const i = onboardingSteps.value.findIndex((s) => !s.done)
  return i < 0 ? 0 : i
})
function dismissOnboarding() {
  onboardDismissed.value = true
  localStorage.setItem('onboardingDismissed', '1')
}

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
  loading.value = true
  try {
    data.value = await api.overview()
    renderChart()
    renderPie()
  } finally {
    loading.value = false
  }
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

// 设备状态变化时刷新统计（debounce：短时间内多条状态变更合并为一次拉取）
const debouncedLoad = debounce(load, 400)
function onMsg(msg: any) {
  if (msg.type === 'device_status') debouncedLoad()
}

const onResize = () => { chart.value?.resize(); pie.value?.resize() }

onMounted(() => {
  load()
  // 平台公告（失败静默，不影响概览）
  api.announcements.list().then((list) => { announcements.value = list || [] }).catch(() => {})
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
.onboard-card { margin-bottom: 16px; border-radius: 10px; border: 1px solid #d9ecff; }
.onboard-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
.onboard-title { display: flex; align-items: center; gap: 8px; font-weight: 600; font-size: 15px; color: #303133; }
.onboard-steps { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }
.onboard-step {
  display: flex; flex-direction: column; gap: 10px; padding: 14px;
  background: #f7f9fc; border-radius: 8px; border: 1px solid transparent;
}
.onboard-step.done { opacity: 0.72; }
.step-dot {
  width: 26px; height: 26px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: #dcdfe6; color: #fff; font-size: 13px; font-weight: 600;
}
.onboard-step.done .step-dot { background: #67c23a; }
.step-name { display: flex; align-items: center; gap: 6px; font-weight: 600; font-size: 14px; color: #303133; }
.step-desc { color: #909399; font-size: 12px; line-height: 1.6; margin-top: 4px; }
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
.announce-card { margin-bottom: 16px; border-radius: 10px; }
.announce-inner { display: flex; align-items: flex-start; gap: 12px; }
.announce-icon { font-size: 22px; margin-top: 2px; }
.announce-body { flex: 1; min-width: 0; }
.announce-item { display: flex; align-items: center; gap: 10px; line-height: 1.9; }
.announce-title { color: #303133; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.announce-title.important { color: #f56c6c; font-weight: 600; }
.announce-meta { color: #c0c4cc; font-size: 12px; flex-shrink: 0; }
</style>
