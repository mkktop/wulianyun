<template>
  <el-card shadow="never">
    <template #header>
      <div class="card-head">
        <span>设备模拟器</span>
        <div class="status-indicator">
          <span class="dot" :class="connected ? 'online' : 'offline'"></span>
          <el-text :type="connected ? 'success' : 'info'" size="small">
            {{ connected ? '已连接' : '未连接' }}
          </el-text>
          <el-tag v-if="sessionId" size="small" type="info" style="margin-left: 8px">
            Session: {{ sessionId.slice(0, 8) }}
          </el-tag>
        </div>
      </div>
    </template>

    <!-- 产品/设备选择器 -->
    <el-form :inline="true" class="selector-row">
      <el-form-item label="产品">
        <el-select v-model="selectedProductId" placeholder="选择产品" filterable style="width: 220px" @change="onProductChange">
          <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="设备">
        <el-select v-model="selectedDeviceId" placeholder="选择设备" filterable style="width: 220px" :disabled="!selectedProductId">
          <el-option v-for="d in devices" :key="d.id" :label="d.name" :value="d.id" />
        </el-select>
      </el-form-item>
      <el-form-item>
        <el-button v-if="!connected" type="primary" :loading="connecting" :disabled="!selectedDeviceId" @click="connect">
          连接
        </el-button>
        <el-button v-else type="danger" :loading="disconnecting" @click="disconnect">断开</el-button>
      </el-form-item>
    </el-form>

    <el-divider />

    <el-row :gutter="20">
      <!-- 左栏：上报面板 -->
      <el-col :span="12">
        <!-- 遥测上报 -->
        <el-card shadow="never" class="inner-card">
          <template #header>
            <div class="card-head">
              <span>遥测上报</span>
              <el-button link type="primary" size="small" @click="applyTelemetryTemplate">填入模板</el-button>
            </div>
          </template>
          <el-input
            v-model="telemetryPayload"
            type="textarea"
            :rows="6"
            placeholder='{"temperature": 25.5, "humidity": 60}'
          />
          <div class="action-bar">
            <el-button type="primary" :loading="publishing" :disabled="!connected" @click="publishTelemetry">
              发送遥测
            </el-button>
          </div>
        </el-card>

        <!-- 事件上报 -->
        <el-card shadow="never" class="inner-card" style="margin-top: 16px">
          <template #header><span>事件上报</span></template>
          <el-form :model="eventForm" label-width="90px" size="small">
            <el-form-item label="标识符">
              <el-input v-model="eventForm.identifier" placeholder="如 alarm_event" />
            </el-form-item>
            <el-form-item label="类型">
              <el-select v-model="eventForm.type" placeholder="选择类型" style="width: 100%">
                <el-option label="info" value="info" />
                <el-option label="alert" value="alert" />
                <el-option label="error" value="error" />
              </el-select>
            </el-form-item>
            <el-form-item label="参数">
              <el-input v-model="eventForm.params" type="textarea" :rows="3" placeholder='{"level": "high", "msg": "温度过高"}' />
            </el-form-item>
          </el-form>
          <div class="action-bar">
            <el-button type="warning" :loading="publishing" :disabled="!connected" @click="publishEvent">
              发送事件
            </el-button>
          </div>
        </el-card>
      </el-col>

      <!-- 右栏：下行消息 -->
      <el-col :span="12">
        <el-card shadow="never" class="inner-card">
          <template #header>
            <div class="card-head">
              <span>下行消息</span>
              <el-button link type="primary" size="small" @click="downlinkMessages = []">清空</el-button>
            </div>
          </template>
          <el-empty v-if="!downlinkMessages.length" description="暂无下行消息" :image-size="80" />
          <el-table v-else :data="downlinkMessages" size="small" max-height="500">
            <el-table-column label="时间" width="160">
              <template #default="{ row }">{{ fmtTime(row.ts) }}</template>
            </el-table-column>
            <el-table-column label="类型" width="90">
              <template #default="{ row }">
                <el-tag size="small" :type="row.type === 'command' ? 'warning' : 'info'">{{ row.type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="内容" min-width="200">
              <template #default="{ row }">
                <el-text size="small" class="payload-text">{{ formatPayload(row.payload) }}</el-text>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type Product, type Device } from '../api'
import { fmtTime } from '../utils/format'

const products = ref<Product[]>([])
const devices = ref<Device[]>([])
const selectedProductId = ref<number | null>(null)
const selectedDeviceId = ref<number | null>(null)
const connected = ref(false)
const connecting = ref(false)
const disconnecting = ref(false)
const sessionId = ref('')
const publishing = ref(false)
const telemetryPayload = ref('')
const downlinkMessages = ref<any[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

const eventForm = reactive({
  identifier: '',
  type: 'info',
  params: ''
})

const telemetryTemplate = JSON.stringify({ temperature: 25.5, humidity: 60, pressure: 1013 }, null, 2)

function applyTelemetryTemplate() {
  telemetryPayload.value = telemetryTemplate
}

async function loadProducts() {
  const res = await api.listProducts({ page: 1, size: 100 })
  products.value = res.list
}

async function onProductChange() {
  selectedDeviceId.value = null
  devices.value = []
  if (!selectedProductId.value) return
  const res = await api.listDevices({ productId: selectedProductId.value, page: 1, size: 200 })
  devices.value = res.list
}

async function connect() {
  if (!selectedProductId.value || !selectedDeviceId.value) return
  connecting.value = true
  try {
    const res = await api.simulator.connect({
      productId: selectedProductId.value,
      deviceId: selectedDeviceId.value
    }) as any
    sessionId.value = res.sessionId
    connected.value = true
    ElMessage.success('连接成功')
    startPolling()
  } finally {
    connecting.value = false
  }
}

async function disconnect() {
  if (!sessionId.value) return
  disconnecting.value = true
  try {
    await api.simulator.disconnect(sessionId.value)
    connected.value = false
    sessionId.value = ''
    stopPolling()
    ElMessage.success('已断开')
  } finally {
    disconnecting.value = false
  }
}

async function publishTelemetry() {
  if (!sessionId.value) return
  let payload: any
  try {
    payload = JSON.parse(telemetryPayload.value)
  } catch {
    ElMessage.warning('请输入合法的 JSON')
    return
  }
  publishing.value = true
  try {
    await api.simulator.publish({ sessionId: sessionId.value, payload })
    ElMessage.success('遥测已发送')
  } finally {
    publishing.value = false
  }
}

async function publishEvent() {
  if (!sessionId.value) return
  if (!eventForm.identifier) {
    ElMessage.warning('请输入事件标识符')
    return
  }
  let params: any = {}
  if (eventForm.params) {
    try { params = JSON.parse(eventForm.params) } catch {
      ElMessage.warning('参数请输入合法 JSON')
      return
    }
  }
  publishing.value = true
  try {
    await api.simulator.publish({
      sessionId: sessionId.value,
      payload: {
        type: 'event',
        identifier: eventForm.identifier,
        eventType: eventForm.type,
        params
      }
    })
    ElMessage.success('事件已发送')
  } finally {
    publishing.value = false
  }
}

function startPolling() {
  pollTimer = setInterval(async () => {
    if (!sessionId.value) return
    try {
      const sessions = await api.simulator.sessions() as any[]
      const cur = sessions.find((s: any) => s.sessionId === sessionId.value)
      if (cur?.downlink?.length) {
        for (const msg of cur.downlink) {
          downlinkMessages.value.unshift({ ts: Date.now(), ...msg })
        }
        if (downlinkMessages.value.length > 200) {
          downlinkMessages.value = downlinkMessages.value.slice(0, 200)
        }
      }
    } catch { /* ignore */ }
  }, 2000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

function formatPayload(p: any) {
  if (typeof p === 'string') return p
  return JSON.stringify(p)
}

onMounted(() => { loadProducts() })
onUnmounted(() => { stopPolling() })
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.status-indicator { display: flex; align-items: center; gap: 6px; }
.dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
.dot.online { background: #67C23A; }
.dot.offline { background: #c0c4cc; }
.selector-row { display: flex; align-items: flex-end; gap: 0; }
.inner-card { border: 1px solid #ebeef5; }
.action-bar { margin-top: 12px; display: flex; justify-content: flex-end; }
.payload-text { font-family: monospace; font-size: 12px; word-break: break-all; }
</style>
