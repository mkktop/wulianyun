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

    <el-tabs v-model="activeTab" style="margin-top: 16px">
      <el-tab-pane label="设备监控" name="monitor">
    <el-row :gutter="16">
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
          <template v-if="!viewOnly && writableProps.length">
            <div v-for="p in writableProps" :key="p.identifier" class="prop-row">
              <div class="prop-info">
                <span class="prop-name">
                  {{ p.name }}
                  <el-text v-if="propRange(p)" type="info" size="small">（{{ propRange(p) }}）</el-text>
                </span>
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
              <el-tag :type="desiredState(p).type" size="small">{{ desiredState(p).text }}</el-tag>
              <el-button size="small" type="primary" :loading="settingProp" @click="setModelProp(p)">设置</el-button>
            </div>
          </template>
          <el-empty v-else-if="!viewOnly" description="产品未定义可写属性，请先在产品管理中配置物模型" :image-size="60" />
          <el-divider style="margin: 12px 0" />
          <el-text type="info" size="small">离线设备写入影子，上线后自动补发；期望达成后自动清除</el-text>
        </el-card>

        <!-- 物模型服务调用 -->
        <el-card v-if="services.length && !viewOnly" shadow="never" style="margin-top: 16px">
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

        <el-card v-if="!viewOnly" shadow="never" style="margin-top: 16px">
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
              <div class="curve-tools">
                <el-radio-group v-model="range" size="small" @change="onRangeChange">
                  <el-radio-button :value="1">1小时</el-radio-button>
                  <el-radio-button :value="6">6小时</el-radio-button>
                  <el-radio-button :value="24">24小时</el-radio-button>
                </el-radio-group>
                <el-date-picker
                  v-model="customRange"
                  type="datetimerange"
                  size="small"
                  range-separator="至"
                  start-placeholder="开始时间"
                  end-placeholder="结束时间"
                  :shortcuts="dateShortcuts"
                  @change="loadHistory"
                />
              </div>
            </div>
          </template>
          <el-empty v-if="!fields.length" description="暂无数值型数据" :image-size="80" />
          <div v-for="f in pagedFields" :key="f" class="field-chart">
            <div class="field-title">{{ propLabel(f) }}<span v-if="propUnit(f)">（{{ propUnit(f) }}）</span></div>
            <div :ref="(el) => setChartEl(f, el as HTMLElement)" class="chart-sm"></div>
          </div>
          <div v-if="chartTotalPages > 1" class="chart-pager">
            <el-button size="small" :disabled="chartPage <= 1" @click="chartPage--; changeChartPage()">上一页</el-button>
            <el-text size="small">{{ chartPage }} / {{ chartTotalPages }}</el-text>
            <el-button size="small" :disabled="chartPage >= chartTotalPages" @click="chartPage++; changeChartPage()">下一页</el-button>
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
      </el-tab-pane>

      <el-tab-pane label="运行日志" name="logs">
        <el-card shadow="never">
          <div class="toolbar">
            <el-select v-model="logCategory" placeholder="全部分类" clearable style="width: 160px" @change="loadLogs">
              <el-option label="连接" value="connection" />
              <el-option label="数据上行" value="data_up" />
              <el-option label="数据下行" value="data_down" />
              <el-option label="事件" value="event" />
              <el-option label="错误" value="error" />
            </el-select>
          </div>
          <el-table :data="logs" v-loading="logsLoading" size="small" stripe>
            <el-table-column type="expand">
              <template #default="{ row }">
                <pre class="log-payload">{{ prettyPayload(row.payload) }}</pre>
              </template>
            </el-table-column>
            <el-table-column label="分类" width="110">
              <template #default="{ row }">
                <el-tag size="small" :type="logCategoryType(row.category)">{{ logCategoryText(row.category) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="summary" label="摘要" width="180" show-overflow-tooltip />
            <el-table-column label="数据" min-width="300">
              <template #default="{ row }">
                <el-text size="small" class="payload-preview">{{ row.payload || '-' }}</el-text>
              </template>
            </el-table-column>
            <el-table-column label="时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
          </el-table>
          <el-pagination
            class="pager" background layout="total, prev, pager, next"
            :total="logsTotal" :page-size="logsSize" v-model:current-page="logsPage" @current-change="loadLogs"
          />
        </el-card>
      </el-tab-pane>

      <el-tab-pane v-if="device?.isGateway" label="子设备" name="subdevices">
        <el-card shadow="never">
          <div class="toolbar">
            <el-button v-if="!viewOnly" type="primary" @click="showAddSub = true">添加子设备</el-button>
          </div>
          <el-table :data="subDevices" v-loading="subLoading" stripe>
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="name" label="设备名称" />
            <el-table-column prop="productKey" label="ProductKey" width="160" />
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column v-if="!viewOnly" label="操作" width="100">
              <template #default="{ row }">
                <el-button link type="danger" @click="removeSub(row.id)">移除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 添加子设备对话框 -->
    <el-dialog v-model="showAddSub" title="添加子设备" width="400px">
      <el-form label-width="80px">
        <el-form-item label="设备ID">
          <el-input-number v-model="subDeviceId" :min="1" style="width: 100%" placeholder="输入要添加的设备ID" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddSub = false">取消</el-button>
        <el-button type="primary" @click="addSub">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import echarts from '../utils/echarts'
import { api, type Device, type TslProperty, type TslService, isViewOnly } from '../api'
import { fmtDateTime } from '../utils/format'
import { realtime } from '../utils/realtime'

const route = useRoute()
const id = Number(route.params.id)
// 只读账号：隐藏所有下行写操作（后端 RequireOperate 兜底）
const viewOnly = isViewOnly()
const device = ref<Device | null>(null)
const latest = ref<Record<string, any>>({})
const lastTs = ref(0)
const events = ref<any[]>([])
const command = ref('')
const sending = ref(false)
const secretVisible = ref(false)
const range = ref(1)
const customRange = ref<[Date, Date] | null>(null)

// 日期选择器快捷选项
const dateShortcuts = [
  { text: '最近1小时', value: () => [new Date(Date.now() - 3600 * 1000), new Date()] },
  { text: '最近6小时', value: () => [new Date(Date.now() - 6 * 3600 * 1000), new Date()] },
  { text: '最近24小时', value: () => [new Date(Date.now() - 24 * 3600 * 1000), new Date()] },
  { text: '今天', value: () => { const d = new Date(); d.setHours(0, 0, 0, 0); return [d, new Date()] } },
  { text: '昨天', value: () => { const s = new Date(); s.setDate(s.getDate() - 1); s.setHours(0, 0, 0, 0); const e = new Date(); e.setDate(e.getDate() - 1); e.setHours(23, 59, 59, 999); return [s, e] } },
]

// 快捷范围切换：清空自定义时间，回到快捷窗口
function onRangeChange() {
  customRange.value = null
  loadHistory()
}
const shadow = ref<any>(null)
const settingProp = ref(false)
const invoking = ref('')
const activeTab = ref('monitor')

// 运行日志
const logCategory = ref('')
const logs = ref<any[]>([])
const logsTotal = ref(0)
const logsPage = ref(1)
const logsSize = 15
const logsLoading = ref(false)

// 子设备
const subDevices = ref<any[]>([])
const subLoading = ref(false)
const showAddSub = ref(false)
const subDeviceId = ref(0)

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

// 曲线：每页最多 2 个变量，翻页查看
const fields = ref<string[]>([])
const chartPage = ref(1)
const chartPageSize = 2
const chartTotalPages = computed(() => Math.max(1, Math.ceil(fields.value.length / chartPageSize)))
const pagedFields = computed(() => fields.value.slice((chartPage.value - 1) * chartPageSize, chartPage.value * chartPageSize))
const series = new Map<string, [number, number][]>()
const chartEls = new Map<string, HTMLElement>()
const charts = new Map<string, echarts.ECharts>()
const palette = ['#409EFF', '#67C23A', '#E6A23C', '#F56C6C', '#9b59b6', '#16a085', '#e67e22', '#2c8fbf']

// 最近设置过的属性（来自命令日志，用于区分"已达成/未设置"）
const setProps = ref<Set<string>>(new Set())

// 每个可写属性的可设置范围文本（bool 0/1、enum 枚举、数值 min~max），text 无范围返回空
function propRange(p: TslProperty) {
  if (p.dataType === 'bool') return '0/1'
  if (p.dataType === 'enum') return (p.enumSpec || []).map((e: any) => e.label || e.value).join('/')
  if (p.dataType === 'int32' || p.dataType === 'float' || p.dataType === 'double') {
    const u = p.unit || ''
    if (p.min != null && p.max != null) return `${p.min} ~ ${p.max}${u}`
    if (p.min != null) return `≥ ${p.min}${u}`
    if (p.max != null) return `≤ ${p.max}${u}`
    return u ? `数值${u}` : ''
  }
  return ''
}

// 每个可写属性的期望设置状态：未达成（desired 有值）/ 已达成（设置过且 desired 已清）/ 未设置
function desiredState(p: TslProperty) {
  const desired = shadow.value?.desired
  const obj = desired ? (typeof desired === 'string' ? JSON.parse(desired) : desired) : {}
  if (obj[p.identifier] !== undefined) {
    return { type: 'warning', text: `期望 ${obj[p.identifier]}（未达成）` }
  }
  if (setProps.value.has(p.identifier)) {
    return { type: 'success', text: '已达成' }
  }
  return { type: 'info', text: '未设置' }
}

function propLabel(key: string) {
  return propMeta.value[key]?.name || key
}
function propUnit(key: string) {
  return propMeta.value[key]?.unit || ''
}

async function loadShadow() {
  shadow.value = await api.getShadow(id)
  // 从命令日志提取最近设置过的属性（判断"已达成/未设置"）
  try {
    const cmds = await api.listCommandLogs({ deviceId: id, size: 30 })
    const s = new Set<string>()
    for (const c of (cmds.list || [])) {
      const p = JSON.parse(c.payload || '{}')
      if (p.method === 'property.set' && p.params) {
        for (const k of Object.keys(p.params)) s.add(k)
      }
    }
    setProps.value = s
  } catch {
    /* 命令日志不可用时不阻塞影子加载 */
  }
}

function prettyPayload(v: string | null) {
  if (!v) return '-'
  try {
    return JSON.stringify(JSON.parse(v), null, 2)
  } catch {
    return v
  }
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
  events.value = (await api.deviceEvents(id, { size: 50 })).list
  if (device.value?.productId) await loadThingModel(device.value.productId)
}

async function loadHistory() {
  let end = Date.now()
  let start = end - range.value * 3600 * 1000
  // 自定义时间范围优先（radio 快捷按钮与日期选择互斥：切换 radio 会清空自定义范围）
  if (customRange.value?.[0] && customRange.value?.[1]) {
    start = customRange.value[0].getTime()
    end = customRange.value[1].getTime()
  }
  const points = await api.deviceHistory(id, { start, end, limit: 5000 })
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

// 曲线翻页：销毁当前页图表实例，重建新页容器
async function changeChartPage() {
  charts.forEach((c) => c.dispose())
  charts.clear()
  chartEls.clear()
  await nextTick()
  renderAll()
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

// 高频 telemetry 时用 requestAnimationFrame 合批渲染：一帧内多条消息只触发一次 renderAll，
// 避免逐条 setOption({notMerge:true}) 造成 ECharts 频繁重绘卡顿。
let renderRaf = 0
let alive = true
function scheduleRender() {
  if (renderRaf || !alive) return
  renderRaf = requestAnimationFrame(() => {
    renderRaf = 0
    if (alive) renderAll()
  })
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
      // 新字段需先同步图表容器，再在下一帧渲染
      syncFields().then(() => scheduleRender())
    } else {
      scheduleRender()
    }
  }
  if (msg.type === 'device_status' && msg.deviceId === id && device.value) {
    device.value.status = msg.payload.status
    api.deviceEvents(id, { size: 50 }).then((e: any) => (events.value = e.list))
  }
}

function statusType(s: string) {
  return ({ online: 'success', offline: 'info', inactive: 'warning', disabled: 'danger' } as any)[s] || 'info'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', inactive: '未激活', disabled: '已禁用' } as any)[s] || s
}
function fmt(s: string | null) {
  return fmtDateTime(s)
}
function copy(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

const onResize = () => charts.forEach((c) => c.resize())

async function loadLogs() {
  logsLoading.value = true
  try {
    const params: any = { page: logsPage.value, size: logsSize }
    if (logCategory.value) params.category = logCategory.value
    const res = await api.deviceLogs.list(id, params) as any
    logs.value = res.list || []
    logsTotal.value = res.total || 0
  } finally {
    logsLoading.value = false
  }
}

function logCategoryType(c: string) {
  return ({ connection: 'success', data_up: '', data_down: 'warning', event: 'info', error: 'danger' } as any)[c] || 'info'
}
function logCategoryText(c: string) {
  return ({ connection: '连接', data_up: '数据上行', data_down: '数据下行', event: '事件', error: '错误' } as any)[c] || c
}

async function loadSubDevices() {
  if (!device.value?.isGateway) return
  subLoading.value = true
  try {
    subDevices.value = (await api.gateway.subDevices(id)) as any[]
  } catch {
    subDevices.value = []
  } finally {
    subLoading.value = false
  }
}

async function addSub() {
  if (!subDeviceId.value) {
    ElMessage.warning('请输入设备ID')
    return
  }
  await api.gateway.addSubDevice(id, { deviceId: subDeviceId.value })
  ElMessage.success('已添加')
  showAddSub.value = false
  subDeviceId.value = 0
  loadSubDevices()
}

async function removeSub(deviceId: number) {
  await ElMessageBox.confirm('确认移除该子设备？', '提示')
  await api.gateway.removeSubDevice(id, deviceId)
  ElMessage.success('已移除')
  loadSubDevices()
}

onMounted(async () => {
  await load()
  loadHistory()
  loadShadow()
  loadLogs()
  loadSubDevices()
  realtime.on(onMsg)
  realtime.subscribe(id)
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  alive = false
  if (renderRaf) cancelAnimationFrame(renderRaf)
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
.card-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.curve-tools { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
.metrics { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
.metric { background: #f7f9fc; border-radius: 8px; padding: 12px 16px; }
.metric-name { color: #999; font-size: 13px; }
.metric-value { font-size: 24px; font-weight: 700; margin-top: 4px; }
.metric-unit { font-size: 13px; color: #999; font-weight: 400; margin-left: 2px; }
.field-chart { margin-bottom: 8px; }
.field-title { font-size: 13px; color: #666; font-weight: 600; padding: 4px 0; }
.chart-sm { height: 170px; }
.chart-pager { display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 8px; }
.prop-row { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.prop-info { flex: 1; display: flex; flex-direction: column; }
.prop-name { font-size: 14px; }
.svc-row { display: flex; flex-wrap: wrap; gap: 8px; }
.toolbar { display: flex; justify-content: space-between; margin-bottom: 12px; }
.pager { margin-top: 12px; justify-content: flex-end; }
.payload-preview {
  display: block;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 12px;
  color: #606266;
}
.log-payload {
  margin: 0;
  padding: 10px 14px;
  background: #f7f9fc;
  border-radius: 6px;
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.6;
  color: #303133;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
