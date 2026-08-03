<template>
  <div v-if="product">
    <el-page-header :content="product.name" @back="$router.push('/products')" style="margin-bottom: 16px" />

    <!-- 产品信息卡 -->
    <el-card shadow="never">
      <div class="info-grid">
        <div class="info-main">
          <h3>{{ product.name }}</h3>
          <div class="addr">
            <div>MQTT 接入地址：{{ host }}:1883</div>
            <div>TCP 接入地址：{{ host }}:9100</div>
          </div>
          <el-button link type="primary" @click="$router.push(`/products/${product.id}/edit`)">编辑</el-button>
        </div>
        <div class="info-item"><div class="label">ProductKey</div><div class="val">{{ product.productKey }}
          <el-button link type="primary" size="small" @click="copy(product.productKey)">复制</el-button></div>
        </div>
        <div class="info-item" v-if="product.secretMode === 'product'">
          <div class="label">ProductSecret</div>
          <div class="val">{{ secretVisible ? product.productSecret : '********' }}
            <el-button link type="primary" size="small" @click="secretVisible = !secretVisible">{{ secretVisible ? '隐藏' : '查看' }}</el-button>
          </div>
        </div>
        <div class="info-item"><div class="label">通信协议</div><div class="val">{{ product.protocol.toUpperCase() }}</div></div>
        <div class="info-item"><div class="label">接入方式</div><div class="val">{{ accessText(product.accessMode) }}</div></div>
        <div class="info-item"><div class="label">密钥模式</div><div class="val">{{ product.secretMode === 'product' ? '一型一密' : '一机一密' }}</div></div>
      </div>
    </el-card>

    <el-tabs v-model="tab" style="margin-top: 16px" @tab-change="onTabChange">
      <!-- 产品概况 -->
      <el-tab-pane label="产品概况" name="overview">
        <el-row :gutter="16">
          <el-col :span="8">
            <el-card shadow="hover"><div class="stat">
              <div class="stat-num">{{ stats.total }}</div><div class="stat-label">设备总数</div>
              <div class="stat-sub">激活 {{ stats.activated }} · 在线 {{ stats.online }}</div>
            </div></el-card>
          </el-col>
          <el-col :span="8">
            <el-card shadow="hover"><div class="stat">
              <div class="stat-num">{{ stats.todayNew }}</div><div class="stat-label">今日新增设备</div>
              <div class="stat-sub">&nbsp;</div>
            </div></el-card>
          </el-col>
          <el-col :span="8">
            <el-card shadow="hover"><div class="stat">
              <div class="stat-num">{{ stats.msgToday }}</div><div class="stat-label">今日消息数</div>
              <div class="stat-sub">&nbsp;</div>
            </div></el-card>
          </el-col>
        </el-row>
        <el-row :gutter="16" style="margin-top: 16px">
          <el-col :span="12">
            <el-card shadow="never"><template #header>设备增长（近14天每日新增）</template>
              <div ref="devTrendRef" class="chart"></div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card shadow="never"><template #header>消息量（近7天）</template>
              <div ref="msgTrendRef" class="chart"></div>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <!-- 功能定义 -->
      <el-tab-pane :label="defLabel" name="definition">
        <el-card shadow="never">
          <div class="toolbar" v-if="product.accessMode === 'thingmodel'">
            <span />
            <div>
              <el-button @click="exportTsl">
                <el-icon><Download /></el-icon>&nbsp;导出 TSL
              </el-button>
              <el-upload
                :show-file-list="false" accept=".json" :before-upload="importTsl"
                style="display: inline-block; margin-left: 8px"
              >
                <el-button>
                  <el-icon><Upload /></el-icon>&nbsp;导入 TSL
                </el-button>
              </el-upload>
            </div>
          </div>
          <ThingModelEditor v-if="product.accessMode === 'thingmodel'"
            v-model:properties="tsl.properties" v-model:events="tsl.events" v-model:services="tsl.services" />
          <ModbusPointEditor v-else-if="product.accessMode === 'modbus'"
            v-model:points="points" :product-id="product.id" />
          <CodecEditor v-else v-model:script="script" :product-id="product.id" />
          <div style="margin-top: 12px; text-align: right">
            <el-button type="primary" :loading="savingDef" @click="saveDefinition">保存</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- 设备管理 -->
      <el-tab-pane label="设备管理" name="devices">
        <el-card shadow="never">
          <div class="toolbar">
            <span />
            <div>
              <el-button @click="batchVisible = true">批量导入</el-button>
              <el-button type="primary" @click="$router.push(`/devices?productId=${product.id}`)">设备管理页</el-button>
            </div>
          </div>
          <el-table :data="devices" size="small" v-loading="devLoading">
            <el-table-column label="设备名称" min-width="140">
              <template #default="{ row }">
                <el-link type="primary" @click="$router.push(`/devices/${row.id}`)">{{ row.name }}</el-link>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="groupName" label="分组" width="120" />
            <el-table-column label="最后上线" width="170">
              <template #default="{ row }">{{ fmt(row.lastOnlineAt) }}</template>
            </el-table-column>
            <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
          </el-table>
          <el-pagination class="pager" background layout="total, prev, pager, next"
            :total="devTotal" :page-size="10" v-model:current-page="devPage" @current-change="loadDevices" />
        </el-card>
      </el-tab-pane>

      <!-- Topic 列表 -->
      <el-tab-pane label="Topic列表" name="topics" v-if="product.protocol === 'mqtt'">
        <el-card shadow="never">
          <el-table :data="topics" size="small">
            <el-table-column prop="topic" label="Topic" min-width="320" />
            <el-table-column prop="dir" label="方向" width="90" />
            <el-table-column prop="desc" label="说明" min-width="220" />
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 事件上报 -->
      <el-tab-pane label="事件上报" name="events">
        <el-card shadow="never">
          <div class="toolbar">
            <el-select v-model="eventType" placeholder="全部类型" clearable style="width: 140px" @change="loadEvents">
              <el-option label="信息" value="info" /><el-option label="告警" value="alert" /><el-option label="故障" value="fault" />
            </el-select>
            <span />
          </div>
          <el-table :data="events" size="small" v-loading="evLoading">
            <el-table-column label="类型" width="80">
              <template #default="{ row }">
                <el-tag :type="evTag(row.type)" size="small">{{ evText(row.type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="deviceName" label="设备" width="130" />
            <el-table-column prop="identifier" label="事件标识符" width="140" />
            <el-table-column label="参数" min-width="220">
              <template #default="{ row }">
                <el-text size="small" type="info">{{ JSON.stringify(row.params) }}</el-text>
              </template>
            </el-table-column>
            <el-table-column label="上报时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
          </el-table>
          <el-pagination class="pager" background layout="total, prev, pager, next"
            :total="evTotal" :page-size="10" v-model:current-page="evPage" @current-change="loadEvents" />
        </el-card>
      </el-tab-pane>

      <!-- 远程配置 -->
      <el-tab-pane label="远程配置" name="config">
        <el-card shadow="never">
          <el-alert type="info" :closable="false" style="margin-bottom: 12px">
            JSON 格式配置，设备可通过 method=config.get 主动拉取；也可点击"推送"广播给产品下全部在线设备（当前版本 v{{ cfgVersion }}）
          </el-alert>
          <el-input v-model="cfgText" type="textarea" :rows="12" placeholder='{"reportInterval": 60}' spellcheck="false" style="font-family: monospace" />
          <div style="margin-top: 12px; display: flex; justify-content: space-between">
            <el-button @click="broadcastVisible = true">自定义广播</el-button>
            <div>
              <el-button type="primary" plain :loading="cfgSaving" @click="saveCfg">保存配置</el-button>
              <el-button type="primary" :loading="cfgPushing" @click="pushCfg">保存并推送</el-button>
            </div>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- 产品下放（仅一级） -->
      <el-tab-pane v-if="isPrimary" label="下放管理" name="grants">
        <el-card shadow="never">
          <el-alert type="info" :closable="false" style="margin-bottom: 12px">
            将该产品下放给名下二级账号，二级即可用此产品类型注册和管理自己的设备（产品定义对二级只读）
          </el-alert>
          <div class="toolbar">
            <span>已下放给 {{ grants.length }} 个二级账号</span>
            <el-button type="primary" @click="openGrant">下放给子账号</el-button>
          </div>
          <el-table :data="grants" size="small" v-loading="grantLoading">
            <el-table-column prop="secondaryName" label="子账号" min-width="140" />
            <el-table-column prop="nickname" label="昵称" min-width="120" />
            <el-table-column prop="permission" label="权限" width="100" />
            <el-table-column label="下放时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="100">
              <template #default="{ row }">
                <el-popconfirm title="确定撤销下放？" @confirm="revokeGrant(row)">
                  <template #reference><el-button link type="danger" size="small">撤销</el-button></template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- 指令下发日志 -->
      <el-tab-pane label="指令下发日志" name="cmdlogs">
        <el-card shadow="never">
          <el-table :data="cmdLogs" size="small" v-loading="logLoading">
            <el-table-column prop="deviceName" label="设备" width="130" />
            <el-table-column label="通道" width="90">
              <template #default="{ row }"><el-tag size="small">{{ row.channel.toUpperCase() }}</el-tag></template>
            </el-table-column>
            <el-table-column label="下发内容" min-width="260">
              <template #default="{ row }">
                <el-text size="small" type="info">{{ row.payload }}</el-text>
              </template>
            </el-table-column>
            <el-table-column label="结果" width="90">
              <template #default="{ row }">
                <el-tag :type="row.success ? 'success' : 'danger'" size="small">{{ row.success ? '成功' : '失败' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="error" label="错误" min-width="140" show-overflow-tooltip />
            <el-table-column label="时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
          </el-table>
          <el-pagination class="pager" background layout="total, prev, pager, next"
            :total="logTotal" :page-size="10" v-model:current-page="logPage" @current-change="loadCmdLogs" />
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 批量导入设备 -->
    <el-dialog v-model="batchVisible" title="批量导入设备" width="480px">
      <el-alert type="info" :closable="false" style="margin-bottom: 12px">每行一个设备名称，单次最多 500 个</el-alert>
      <el-input v-model="batchNames" type="textarea" :rows="10" placeholder="dev-001&#10;dev-002&#10;dev-003" />
      <template #footer>
        <el-button @click="batchVisible = false">取消</el-button>
        <el-button type="primary" :loading="importing" @click="doImport">导入</el-button>
      </template>
    </el-dialog>

    <!-- 自定义广播 -->
    <el-dialog v-model="broadcastVisible" title="自定义广播" width="520px">
      <el-alert type="warning" :closable="false" style="margin-bottom: 12px">
        JSON 消息将下发到产品下全部在线设备（MQTT 设备需订阅 thing/broadcast/{{ product.productKey }}）
      </el-alert>
      <el-input v-model="broadcastText" type="textarea" :rows="8" placeholder='{"method":"custom.notify","params":{}}' spellcheck="false" style="font-family: monospace" />
      <template #footer>
        <el-button @click="broadcastVisible = false">取消</el-button>
        <el-button type="primary" :loading="broadcasting" @click="doBroadcast">广播</el-button>
      </template>
    </el-dialog>

    <!-- 下放产品 -->
    <el-dialog v-model="grantVisible" title="下放产品给子账号" width="420px">
      <el-form label-width="80px">
        <el-form-item label="子账号">
          <el-select v-model="grantSecondaryId" placeholder="选择名下二级账号" style="width: 100%">
            <el-option v-for="a in accounts" :key="a.id"
              :label="a.nickname ? `${a.nickname}（${a.username}）` : a.username" :value="a.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="grantVisible = false">取消</el-button>
        <el-button type="primary" :loading="granting" @click="doGrant">下放</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as echarts from 'echarts'
import {
  api, type Product, type Device, type EventReport, type CommandLog,
  type ModbusPoint, type TslProperty, type TslEvent, type TslService,
  currentTier, type Account, type ProductGrant
} from '../api'
import ThingModelEditor from '../components/ThingModelEditor.vue'
import ModbusPointEditor from '../components/ModbusPointEditor.vue'
import CodecEditor from '../components/CodecEditor.vue'

const route = useRoute()
const id = Number(route.params.id)
const host = window.location.hostname
const product = ref<Product | null>(null)
const secretVisible = ref(false)
const tab = ref('overview')

// 概况
const stats = ref<any>({ total: 0, activated: 0, online: 0, todayNew: 0, msgToday: 0, deviceTrend: [], msgTrend: [] })
const devTrendRef = ref<HTMLElement>()
const msgTrendRef = ref<HTMLElement>()
const devTrendChart = shallowRef<echarts.ECharts>()
const msgTrendChart = shallowRef<echarts.ECharts>()

// 功能定义
const tsl = reactive<{ properties: TslProperty[]; events: TslEvent[]; services: TslService[] }>({ properties: [], events: [], services: [] })
const points = ref<ModbusPoint[]>([])
const script = ref('')
const savingDef = ref(false)
const defLoaded = ref(false)

// 设备
const devices = ref<Device[]>([])
const devTotal = ref(0)
const devPage = ref(1)
const devLoading = ref(false)
const batchVisible = ref(false)
const batchNames = ref('')
const importing = ref(false)

// 事件 / 日志
const events = ref<EventReport[]>([])
const evTotal = ref(0)
const evPage = ref(1)
const evLoading = ref(false)
const eventType = ref('')
const cmdLogs = ref<CommandLog[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logLoading = ref(false)

// 远程配置 / 广播
const cfgText = ref('{}')
const cfgVersion = ref(0)
const cfgLoaded = ref(false)
const cfgSaving = ref(false)
const cfgPushing = ref(false)
const broadcastVisible = ref(false)
const broadcastText = ref('')
const broadcasting = ref(false)

// 产品下放（仅一级）
const isPrimary = currentTier() === 'primary'
const grants = ref<ProductGrant[]>([])
const grantLoading = ref(false)
const grantVisible = ref(false)
const granting = ref(false)
const grantSecondaryId = ref<number | undefined>(undefined)
const accounts = ref<Account[]>([])

const defLabel = computed(() => {
  if (!product.value) return '功能定义'
  return ({ thingmodel: '功能定义(物模型)', modbus: 'Modbus点位表', passthrough: '解析脚本' } as any)[product.value.accessMode]
})

function accessText(m: string) {
  return ({ thingmodel: '物模型', passthrough: '透传解析', modbus: 'Modbus' } as any)[m] || m
}
function statusType(s: string) {
  return ({ online: 'success', offline: 'info', inactive: 'warning', disabled: 'danger' } as any)[s] || 'info'
}
function statusText(s: string) {
  return ({ online: '在线', offline: '离线', inactive: '未激活', disabled: '已禁用' } as any)[s] || s
}
function evTag(t: string) {
  return ({ info: 'info', alert: 'warning', fault: 'danger' } as any)[t] || 'info'
}
function evText(t: string) {
  return ({ info: '信息', alert: '告警', fault: '故障' } as any)[t] || t
}
function fmt(s: string | null) {
  return s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
}
function copy(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}
function toArr(v: any) {
  if (!v) return []
  return typeof v === 'string' ? JSON.parse(v) : v
}

// MQTT Topic 约定表
const topics = computed(() => {
  if (!product.value) return []
  const pk = product.value.productKey
  return [
    { topic: `thing/up/${pk}/{deviceName}`, dir: '上行', desc: '属性上报（JSON）；含 method=event.post 时为事件上报' },
    { topic: `thing/down/${pk}/{deviceName}`, dir: '下行', desc: '属性设置/服务调用/透传命令' },
    { topic: `thing/broadcast/${pk}`, dir: '下行', desc: '产品级广播（远程配置推送/自定义广播）' }
  ]
})

async function loadStats() {
  stats.value = await api.productStats(id)
  await nextTick()
  renderTrends()
}

function renderTrends() {
  if (devTrendRef.value) {
    if (!devTrendChart.value) devTrendChart.value = echarts.init(devTrendRef.value)
    const t = stats.value.deviceTrend || []
    devTrendChart.value.setOption({
      grid: { left: 40, right: 16, top: 20, bottom: 26 },
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: t.map((x: any) => x.day) },
      yAxis: { type: 'value', minInterval: 1 },
      series: [{ type: 'bar', barMaxWidth: 28, itemStyle: { color: '#409EFF', borderRadius: [3, 3, 0, 0] }, data: t.map((x: any) => x.count) }]
    })
  }
  if (msgTrendRef.value) {
    if (!msgTrendChart.value) msgTrendChart.value = echarts.init(msgTrendRef.value)
    const t = stats.value.msgTrend || []
    msgTrendChart.value.setOption({
      grid: { left: 50, right: 16, top: 20, bottom: 26 },
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: t.map((x: any) => x.day) },
      yAxis: { type: 'value' },
      series: [{ type: 'line', smooth: true, showSymbol: false, areaStyle: { opacity: 0.15 }, data: t.map((x: any) => x.count) }]
    })
  }
}

async function loadDefinition() {
  if (defLoaded.value || !product.value) return
  defLoaded.value = true
  if (product.value.accessMode === 'thingmodel') {
    const tm = await api.getThingModel(id)
    tsl.properties = toArr(tm.properties)
    tsl.events = toArr(tm.events)
    tsl.services = toArr(tm.services)
  } else if (product.value.accessMode === 'modbus') {
    points.value = await api.getModbusPoints(id)
  } else {
    const cRes = await api.getCodec(id)
    script.value = cRes.script || ''
  }
}

async function saveDefinition() {
  savingDef.value = true
  try {
    if (product.value!.accessMode === 'thingmodel') {
      await api.saveThingModel(id, { properties: tsl.properties, events: tsl.events, services: tsl.services })
    } else if (product.value!.accessMode === 'modbus') {
      await api.saveModbusPoints(id, points.value)
    } else {
      await api.saveCodec(id, script.value)
    }
    ElMessage.success('已保存')
  } finally {
    savingDef.value = false
  }
}

// ---- 物模型导入/导出 ----
const tslImporting = ref(false)

async function exportTsl() {
  try {
    const resp = await api.tsl.export(id) as any
    const blob = new Blob([JSON.stringify(resp, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `tsl-${product.value?.productKey || id}.json`
    a.click()
    URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch {
    ElMessage.error('导出失败')
  }
}

function importTsl(file: File) {
  const reader = new FileReader()
  reader.onload = async (e) => {
    try {
      JSON.parse(e.target?.result as string) // validate JSON
    } catch {
      ElMessage.error('文件格式错误，不是有效的 JSON')
      return
    }
    tslImporting.value = true
    try {
      await api.tsl.import(id, file)
      ElMessage.success('导入成功，正在刷新...')
      // 重新加载物模型
      defLoaded.value = false
      loadDefinition()
    } finally {
      tslImporting.value = false
    }
  }
  reader.readAsText(file)
  return false // prevent el-upload default upload
}

async function loadDevices() {
  devLoading.value = true
  try {
    const res = await api.listDevices({ page: devPage.value, size: 10, productId: id })
    devices.value = res.list
    devTotal.value = res.total
  } finally {
    devLoading.value = false
  }
}

async function loadEvents() {
  evLoading.value = true
  try {
    const res = await api.listEventReports({ page: evPage.value, size: 10, productId: id, type: eventType.value })
    events.value = res.list
    evTotal.value = res.total
  } finally {
    evLoading.value = false
  }
}

async function loadCmdLogs() {
  logLoading.value = true
  try {
    const res = await api.listCommandLogs({ page: logPage.value, size: 10, productId: id })
    cmdLogs.value = res.list
    logTotal.value = res.total
  } finally {
    logLoading.value = false
  }
}

function onTabChange(name: string | number) {
  if (name === 'definition') loadDefinition()
  else if (name === 'devices') loadDevices()
  else if (name === 'events') loadEvents()
  else if (name === 'cmdlogs') loadCmdLogs()
  else if (name === 'config') loadCfg()
  else if (name === 'overview') loadStats()
  else if (name === 'grants') loadGrants()
}

// ---- 产品下放 ----
async function loadGrants() {
  grantLoading.value = true
  try {
    grants.value = await api.listGrants(id)
  } finally {
    grantLoading.value = false
  }
}
async function openGrant() {
  grantSecondaryId.value = undefined
  accounts.value = await api.listAccounts()
  grantVisible.value = true
}
async function doGrant() {
  if (!grantSecondaryId.value) {
    ElMessage.warning('请选择子账号')
    return
  }
  granting.value = true
  try {
    await api.createGrant(id, grantSecondaryId.value)
    ElMessage.success('已下放')
    grantVisible.value = false
    loadGrants()
  } finally {
    granting.value = false
  }
}
async function revokeGrant(row: ProductGrant) {
  await api.deleteGrant(id, row.secondaryId)
  ElMessage.success('已撤销下放')
  loadGrants()
}

// ---- 远程配置 / 广播 ----
async function loadCfg() {
  if (cfgLoaded.value) return
  cfgLoaded.value = true
  const res = await api.getRemoteConfig(id)
  cfgVersion.value = res.version
  cfgText.value = JSON.stringify(res.config || {}, null, 2)
}

function parseCfg(): Record<string, any> | null {
  try {
    const obj = JSON.parse(cfgText.value || '{}')
    if (typeof obj !== 'object' || Array.isArray(obj)) throw new Error()
    return obj
  } catch {
    ElMessage.warning('配置必须是合法的 JSON 对象')
    return null
  }
}

async function saveCfg() {
  const cfg = parseCfg()
  if (!cfg) return
  cfgSaving.value = true
  try {
    const res = await api.saveRemoteConfig(id, cfg)
    cfgVersion.value = res.version
    ElMessage.success(`已保存（v${res.version}）`)
  } finally {
    cfgSaving.value = false
  }
}

async function pushCfg() {
  const cfg = parseCfg()
  if (!cfg) return
  cfgPushing.value = true
  try {
    const res = await api.saveRemoteConfig(id, cfg)
    cfgVersion.value = res.version
    await api.pushRemoteConfig(id)
    ElMessage.success(`已推送到全部在线设备（v${res.version}）`)
  } finally {
    cfgPushing.value = false
  }
}

async function doBroadcast() {
  let payload: any
  try {
    payload = JSON.parse(broadcastText.value)
    if (typeof payload !== 'object' || Array.isArray(payload) || !Object.keys(payload).length) throw new Error()
  } catch {
    ElMessage.warning('广播内容必须是非空 JSON 对象')
    return
  }
  broadcasting.value = true
  try {
    await api.broadcastProduct(id, payload)
    broadcastVisible.value = false
    ElMessage.success('广播已下发')
  } finally {
    broadcasting.value = false
  }
}

async function doImport() {
  const names = batchNames.value.split('\n').map((s) => s.trim()).filter(Boolean)
  if (!names.length) {
    ElMessage.warning('请输入设备名称')
    return
  }
  importing.value = true
  try {
    const res = await api.batchCreateDevices(id, names)
    batchVisible.value = false
    batchNames.value = ''
    loadDevices()
    loadStats()
    if (res.failed?.length) {
      ElMessageBox.alert(
        `成功 ${res.created} 个，失败 ${res.failed.length} 个：<br/>` +
        res.failed.map((f) => `${f.name}（${f.reason}）`).join('<br/>'),
        '导入结果', { dangerouslyUseHTMLString: true }
      )
    } else {
      ElMessage.success(`成功导入 ${res.created} 个设备`)
    }
  } finally {
    importing.value = false
  }
}

const onResize = () => { devTrendChart.value?.resize(); msgTrendChart.value?.resize() }

onMounted(async () => {
  product.value = await api.getProduct(id)
  loadStats()
  window.addEventListener('resize', onResize)
})
onUnmounted(() => {
  window.removeEventListener('resize', onResize)
  devTrendChart.value?.dispose()
  msgTrendChart.value?.dispose()
})
</script>

<style scoped>
.info-grid { display: flex; gap: 32px; align-items: flex-start; flex-wrap: wrap; }
.info-main { min-width: 260px; }
.info-main h3 { margin-bottom: 8px; }
.addr { color: #666; font-size: 13px; line-height: 1.8; margin-bottom: 4px; }
.info-item .label { color: #999; font-size: 13px; margin-bottom: 6px; }
.info-item .val { font-weight: 600; font-size: 14px; }
.stat { text-align: center; }
.stat-num { font-size: 30px; font-weight: 700; color: #409EFF; }
.stat-label { color: #666; margin-top: 4px; }
.stat-sub { color: #999; font-size: 12px; margin-top: 4px; }
.chart { height: 260px; }
.toolbar { display: flex; justify-content: space-between; margin-bottom: 12px; }
.pager { margin-top: 12px; justify-content: flex-end; }
</style>
