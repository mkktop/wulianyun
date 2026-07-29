<template>
  <div v-if="device">
    <el-card shadow="never">
      <div class="head">
        <div>
          <h3>
            {{ device.name }}
            <el-tag :type="statusType(device.status)" style="margin-left: 8px">{{ statusText(device.status) }}</el-tag>
          </h3>
          <div class="sub">所属产品：{{ device.productName }}</div>
        </div>
        <el-button @click="$router.back()">返回</el-button>
      </div>
      <el-descriptions :column="2" border style="margin-top: 12px">
        <el-descriptions-item label="ProductKey">{{ device.productKey }}</el-descriptions-item>
        <el-descriptions-item label="DeviceName">{{ device.name }}</el-descriptions-item>
        <el-descriptions-item label="DeviceSecret">
          <el-text>{{ secretVisible ? device.secret : '********' }}</el-text>
          <el-button link type="primary" @click="secretVisible = !secretVisible">
            {{ secretVisible ? '隐藏' : '查看' }}
          </el-button>
          <el-button link type="primary" @click="copy(device.secret)">复制</el-button>
        </el-descriptions-item>
        <el-descriptions-item label="MQTT ClientID">{{ device.productKey }}.{{ device.name }}</el-descriptions-item>
        <el-descriptions-item label="上报主题">thing/up/{{ device.productKey }}/{{ device.name }}</el-descriptions-item>
        <el-descriptions-item label="下发主题">thing/down/{{ device.productKey }}/{{ device.name }}</el-descriptions-item>
        <el-descriptions-item label="最后上线">{{ fmt(device.lastOnlineAt) }}</el-descriptions-item>
        <el-descriptions-item label="最后离线">{{ fmt(device.lastOfflineAt) }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="10">
        <el-card shadow="never">
          <template #header>
            <div class="card-head">
              <span>实时数据</span>
              <el-tag v-if="lastTs" size="small" type="info">更新于 {{ new Date(lastTs).toLocaleTimeString() }}</el-tag>
            </div>
          </template>
          <el-empty v-if="!Object.keys(latest).length" description="暂无数据" :image-size="80" />
          <div v-else class="metrics">
            <div v-for="(v, k) in latest" :key="k" class="metric">
              <div class="metric-name">{{ propLabel(String(k)) }}</div>
              <div class="metric-value">
                {{ typeof v === 'number' ? +v.toFixed(2) : v }}
                <span class="metric-unit">{{ propUnit(String(k)) }}</span>
              </div>
            </div>
          </div>
        </el-card>

        <!-- 物模型属性设置（读写属性） -->
        <el-card shadow="never" style="margin-top: 16px">
          <template #header>
            <div class="card-head">
              <span>属性设置（物模型）</span>
              <el-button link type="primary" size="small" @click="loadShadow">刷新影子</el-button>
            </div>
          </template>
          <template v-if="writableProps.length">
            <div v-for="p in writableProps" :key="p.identifier" class="prop-row">
              <div class="prop-info">
                <span class="prop-name">{{ p.name }}</span>
                <el-text type="info" size="small">{{ p.identifier }}{{ p.unit ? ' · ' + p.unit : '' }}</el-text>
              </div>
              <el-switch v-if="p.dataType === 'bool'" v-model="propInputs[p.identifier]" />
              <el-select v-else-if="p.dataType === 'enum'" v-model="propInputs[p.identifier]" style="width: 120px">
                <el-option v-for="it in (p.enumSpec || [])" :key="it.value" :label="it.label" :value="it.value" />
              </el-select>
              <el-input-number
                v-else-if="p.dataType === 'int32' || p.dataType === 'float' || p.dataType === 'double'"
                v-model="propInputs[p.identifier]" :controls="false"
                :min="p.min ?? undefined" :max="p.max ?? undefined" :step="p.step ?? undefined"
                style="width: 120px"
              />
              <el-input v-else v-model="propInputs[p.identifier]" style="width: 120px" />
              <el-button size="small" type="primary" :loading="settingProp" @click="setModelProp(p)">设置</el-button>
            </div>
          </template>
          <el-empty v-else description="产品未定义可写属性，请先在产品管理中配置物模型" :image-size="60" />
          <el-divider style="margin: 12px 0" />
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="期望值 desired">
              <el-text size="small">{{ shadowDesiredText }}</el-text>
            </el-descriptions-item>
            <el-descriptions-item label="上报值 reported">
              <el-text size="small">{{ shadowReportedText }}</el-text>
            </el-descriptions-item>
          </el-descriptions>
          <el-text type="info" size="small">离线设备写入影子，上线后自动补发</el-text>
        </el-card>

        <!-- 物模型服务调用 -->
        <el-card v-if="services.length" shadow="never" style="margin-top: 16px">
          <template #header>服务调用（物模型）</template>
          <div class="svc-row">
            <el-button
              v-for="s in services" :key="s.identifier"
              type="warning" plain :loading="invoking === s.identifier" @click="invokeSvc(s)"
            >
              {{ s.name || s.identifier }}
            </el-button>
          </div>
        </el-card>

        <el-card shadow="never" style="margin-top: 16px">
          <template #header>命令下发（原始 JSON 调试）</template>
          <el-input v-model="command" type="textarea" :rows="4" placeholder='{"switch": 1}' />
          <el-button type="primary" style="margin-top: 12px" :loading="sending" @click="send">下发</el-button>
        </el-card>
      </el-col>

      <el-col :span="14">
        <el-card shadow="never">
          <template #header>
            <div class="card-head">
              <span>历史曲线（每个变量独立图表）</span>
              <el-radio-group v-model="range" size="small" @change="loadHistory">
                <el-radio-button :value="1">1小时</el-radio-button>
                <el-radio-button :value="6">6小时</el-radio-button>
                <el-radio-button :value="24">24小时</el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <el-empty v-if="!fields.length" description="暂无数值型数据" :image-size="80" />
          <div v-for="f in fields" :key="f" class="field-chart">
            <div class="field-title">{{ propLabel(f) }}<span v-if="propUnit(f)">（{{ propUnit(f) }}）</span></div>
            <div :ref="(el) => setChartEl(f, el as HTMLElement)" class="chart-sm"></div>
          </div>
        </el-card>

        <el-card shadow="never" style="margin-top: 16px">
          <template #header>事件日志</template>
          <el-table :data="events" size="small" max-height="240">
            <el-table-column label="类型" width="100">
              <template #default="{ row }">
                <el-tag :type="row.type === 'online' ? 'success' : 'info'" size="small">
                  {{ row.type === 'online' ? '上线' : '离线' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="detail" label="详情" />
            <el-table-column label="时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { api, type Device, type TslProperty, type TslService } from '../api'
import { realtime } from '../utils/realtime'

const route = useRoute()
const id = Number(route.params.id)
const device = ref<Device | null>(null)
const latest = ref<Record<string, any>>({})
const lastTs = ref(0)
const events = ref<any[]>([])
const command = ref('')
const sending = ref(false)
const secretVisible = ref(false)
const range = ref(1)
const shadow = ref<any>(null)
const settingProp = ref(false)
const invoking = ref('')

// 物模型
const properties = ref<TslProperty[]>([])
const services = ref<TslService[]>([])
const propInputs = reactive<Record<string, any>>({})
const writableProps = computed(() => properties.value.filter((p) => p.accessMode === 'rw'))
const propMeta = computed(() => {
  const m: Record<string, TslProperty> = {}
  for (const p of properties.value) m[p.identifier] = p
  return m
})

// 曲线：每个变量一张图
const fields = ref<string[]>([])
const series = new Map<string, [number, number][]>()
const chartEls = new Map<string, HTMLElement>()
const charts = new Map<string, echarts.ECharts>()
const palette = ['#409EFF', '#67C23A', '#E6A23C', '#F56C6C', '#9b59b6', '#16a085', '#e67e22', '#2c8fbf']

const shadowDesiredText = computed(() => jsonText(shadow.value?.desired))
const shadowReportedText = computed(() => jsonText(shadow.value?.reported))

function propLabel(key: string) {
  return propMeta.value[key]?.name || key
}
function propUnit(key: string) {
  return propMeta.value[key]?.unit || ''
}

function jsonText(v: any) {
  if (!v) return '{}'
  const obj = typeof v === 'string' ? JSON.parse(v) : v
  return JSON.stringify(obj)
}

async function loadShadow() {
  shadow.value = await api.getShadow(id)
}

async function loadThingModel(productId: number) {
  const tm = await api.getThingModel(productId)
  properties.value = (typeof tm.properties === 'string' ? JSON.parse(tm.properties as any) : tm.properties) || []
  services.value = (typeof tm.services === 'string' ? JSON.parse(tm.services as any) : tm.services) || []
  // 属性设置输入框默认值取当前上报值
  for (const p of writableProps.value) {
    if (propInputs[p.identifier] === undefined) {
      const cur = latest.value[p.identifier]
      propInputs[p.identifier] = p.dataType === 'bool' ? Boolean(cur) : cur ?? (p.dataType === 'text' ? '' : 0)
    }
  }
}

async function setModelProp(p: TslProperty) {
  let v = propInputs[p.identifier]
  if (p.dataType === 'int32') v = Math.round(Number(v))
  if (p.dataType === 'float' || p.dataType === 'double') v = Number(v)
  settingProp.value = true
  try {
    const res = await api.setProperty(id, { [p.identifier]: v })
    ElMessage.success(res.note || '已下发')
    loadShadow()
  } finally {
    settingProp.value = false
  }
}

async function invokeSvc(s: TslService) {
  invoking.value = s.identifier
  try {
    await api.invokeService(id, s.identifier, {})
    ElMessage.success(`服务 ${s.name || s.identifier} 已调用`)
  } finally {
    invoking.value = ''
  }
}

async function load() {
  device.value = await api.getDevice(id)
  const l = await api.deviceLatest(id)
  if (l?.data) {
    latest.value = l.data
    lastTs.value = l.ts
  }
  events.value = await api.deviceEvents(id)
  if (device.value?.productId) await loadThingModel(device.value.productId)
}

async function loadHistory() {
  const end = Date.now()
  const start = end - range.value * 3600 * 1000
  const points = await api.deviceHistory(id, { start, end })
  series.clear()
  for (const p of points) {
    for (const [k, v] of Object.entries(p.data)) {
      if (typeof v !== 'number') continue
      if (!series.has(k)) series.set(k, [])
      series.get(k)!.push([p.ts, v])
    }
  }
  await syncFields()
  renderAll()
}

// 字段集合变化时同步图表容器（新增/移除变量）
async function syncFields() {
  const names = [...series.keys()].sort()
  const removed = fields.value.filter((f) => !names.includes(f))
  for (const f of removed) {
    charts.get(f)?.dispose()
    charts.delete(f)
    chartEls.delete(f)
  }
  fields.value = names
  await nextTick()
}

function setChartEl(field: string, el: HTMLElement | null) {
  if (el) chartEls.set(field, el)
}

function renderField(field: string, index: number) {
  const el = chartEls.get(field)
  if (!el) return
  let chart = charts.get(field)
  if (!chart) {
    chart = echarts.init(el)
    charts.set(field, chart)
  }
  const color = palette[index % palette.length]
  chart.setOption({
    grid: { left: 56, right: 16, top: 10, bottom: 22 },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (v: any) => `${v}${propUnit(field)}`
    },
    xAxis: { type: 'time', axisLabel: { fontSize: 11 } },
    yAxis: { type: 'value', scale: true, axisLabel: { fontSize: 11 } },
    series: [{
      name: propLabel(field), type: 'line', showSymbol: false, smooth: true,
      lineStyle: { color, width: 2 },
      areaStyle: { color, opacity: 0.08 },
      data: series.get(field)
    }]
  }, { notMerge: true })
}

function renderAll() {
  fields.value.forEach((f, i) => renderField(f, i))
}

async function send() {
  try {
    JSON.parse(command.value)
  } catch {
    ElMessage.warning('请输入合法的 JSON')
    return
  }
  sending.value = true
  try {
    await api.sendCommand(id, JSON.parse(command.value))
    ElMessage.success('下发成功')
  } finally {
    sending.value = false
  }
}

function onMsg(msg: any) {
  if (msg.type === 'telemetry' && msg.deviceId === id) {
    latest.value = { ...latest.value, ...msg.payload.data }
    lastTs.value = msg.payload.ts
    let hasNew = false
    for (const [k, v] of Object.entries(msg.payload.data)) {
      if (typeof v !== 'number') continue
      if (!series.has(k)) {
        series.set(k, [])
        hasNew = true
      }
      series.get(k)!.push([msg.payload.ts, v])
    }
    if (hasNew) {
      syncFields().then(renderAll)
    } else {
      fields.value.forEach((f, i) => {
        if (msg.payload.data[f] !== undefined) renderField(f, i)
      })
    }
  }
  if (msg.type === 'device_status' && msg.deviceId === id && device.value) {
    device.value.status = msg.payload.status
    api.deviceEvents(id).then((e) => (events.value = e))
  }
}

function statusType(s: string) {
  return ({ online: 'success', offline: 'info', inactive: 'warning', disabled: 'danger' } as any)[s] || 'info'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', inactive: '未激活', disabled: '已禁用' } as any)[s] || s
}
function fmt(s: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function copy(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

const onResize = () => charts.forEach((c) => c.resize())

onMounted(async () => {
  await load()
  loadHistory()
  loadShadow()
  realtime.on(onMsg)
  realtime.subscribe(id)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  realtime.unsubscribe(id)
  realtime.off(onMsg)
  window.removeEventListener('resize', onResize)
  charts.forEach((c) => c.dispose())
  charts.clear()
})
</script>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: flex-start; }
.head h3 { display: flex; align-items: center; }
.sub { color: #999; font-size: 13px; margin-top: 6px; }
.card-head { display: flex; justify-content: space-between; align-items: center; }
.metrics { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
.metric { background: #f7f9fc; border-radius: 8px; padding: 12px 16px; }
.metric-name { color: #999; font-size: 13px; }
.metric-value { font-size: 24px; font-weight: 700; margin-top: 4px; }
.metric-unit { font-size: 13px; color: #999; font-weight: 400; margin-left: 2px; }
.field-chart { margin-bottom: 8px; }
.field-title { font-size: 13px; color: #666; font-weight: 600; padding: 4px 0; }
.chart-sm { height: 170px; }
.prop-row { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.prop-info { flex: 1; display: flex; flex-direction: column; }
.prop-name { font-size: 14px; }
.svc-row { display: flex; flex-wrap: wrap; gap: 8px; }
</style>
