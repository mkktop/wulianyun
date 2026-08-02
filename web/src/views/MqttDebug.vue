<template>
  <el-card shadow="never">
    <template #header>
      <div class="card-head">
        <span>MQTT 调试台</span>
        <div class="ws-status">
          <span class="dot" :class="wsConnected ? 'online' : 'offline'"></span>
          <el-text :type="wsConnected ? 'success' : 'info'" size="small">
            {{ wsConnected ? '已连接' : '未连接' }}
          </el-text>
          <el-button v-if="!wsConnected" type="primary" size="small" style="margin-left: 12px" @click="connectWs">
            连接
          </el-button>
          <el-button v-else type="danger" size="small" style="margin-left: 12px" @click="disconnectWs">
            断开
          </el-button>
        </div>
      </div>
    </template>

    <el-row :gutter="20">
      <!-- 左栏：发布 + 订阅 -->
      <el-col :span="10">
        <!-- 发布区 -->
        <el-card shadow="never" class="inner-card">
          <template #header><span>发布消息</span></template>
          <el-form label-width="60px" size="small">
            <el-form-item label="Topic">
              <el-input v-model="pubTopic" placeholder="thing/up/productKey/deviceName" />
            </el-form-item>
            <el-form-item label="QoS">
              <el-radio-group v-model="pubQos">
                <el-radio-button :value="0">QoS 0</el-radio-button>
                <el-radio-button :value="1">QoS 1</el-radio-button>
                <el-radio-button :value="2">QoS 2</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="Payload">
              <el-input v-model="pubPayload" type="textarea" :rows="5" placeholder='{"temperature": 25.5}' />
            </el-form-item>
          </el-form>
          <div class="action-bar">
            <el-button type="primary" :disabled="!wsConnected" @click="publish">发送</el-button>
          </div>
        </el-card>

        <!-- 订阅区 -->
        <el-card shadow="never" class="inner-card" style="margin-top: 16px">
          <template #header><span>订阅管理</span></template>
          <div class="sub-add">
            <el-input v-model="subTopic" placeholder="输入 Topic，支持 +/# 通配符" style="flex: 1" />
            <el-button type="primary" :disabled="!wsConnected" @click="subscribe">订阅</el-button>
          </div>
          <el-empty v-if="!subscriptions.length" description="暂无订阅" :image-size="60" />
          <div v-else class="sub-list">
            <el-tag
              v-for="(s, i) in subscriptions" :key="i"
              closable @close="unsubscribe(i)"
              type="info" style="margin: 0 6px 6px 0"
            >
              {{ s }}
            </el-tag>
          </div>
        </el-card>
      </el-col>

      <!-- 右栏：消息流 -->
      <el-col :span="14">
        <el-card shadow="never" class="inner-card">
          <template #header>
            <div class="card-head">
              <span>消息流</span>
              <div>
                <el-checkbox v-model="autoScroll" size="small">自动滚动</el-checkbox>
                <el-button link type="primary" size="small" @click="messages = []" style="margin-left: 12px">清空</el-button>
              </div>
            </div>
          </template>
          <el-empty v-if="!messages.length" description="暂无消息" :image-size="80" />
          <div v-else ref="msgContainer" class="msg-stream">
            <div v-for="(m, i) in messages" :key="i" class="msg-item" :class="m.direction">
              <div class="msg-header">
                <el-tag size="small" :type="m.direction === 'in' ? 'success' : 'warning'" effect="plain">
                  {{ m.direction === 'in' ? '收到' : '发送' }}
                </el-tag>
                <el-text size="small" type="info" style="margin-left: 8px">{{ m.time }}</el-text>
                <el-text size="small" style="margin-left: 8px">QoS {{ m.qos }}</el-text>
                <el-text size="small" class="msg-topic">{{ m.topic }}</el-text>
              </div>
              <div class="msg-payload">{{ m.payload }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </el-card>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const wsConnected = ref(false)
let ws: WebSocket | null = null

// 发布
const pubTopic = ref('')
const pubQos = ref(0)
const pubPayload = ref('')

// 订阅
const subTopic = ref('')
const subscriptions = ref<string[]>([])

// 消息流
const messages = ref<any[]>([])
const msgContainer = ref<HTMLElement | null>(null)
const autoScroll = ref(true)

function getWsUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}${api.mqttDebug.wsUrl}`
}

function connectWs() {
  try {
    ws = new WebSocket(getWsUrl())
    ws.onopen = () => {
      wsConnected.value = true
      ElMessage.success('WebSocket 已连接')
    }
    ws.onmessage = (evt) => {
      try {
        const data = JSON.parse(evt.data)
        if (data.direction) {
          messages.value.push({
            direction: data.direction,
            topic: data.topic,
            payload: data.payload,
            qos: data.qos ?? 0,
            time: new Date().toLocaleTimeString('zh-CN', { hour12: false })
          })
          trimMessages()
          scrollBottom()
        }
      } catch { /* ignore */ }
    }
    ws.onclose = () => {
      wsConnected.value = false
      subscriptions.value = []
    }
    ws.onerror = () => {
      ElMessage.error('WebSocket 连接失败')
      wsConnected.value = false
    }
  } catch {
    ElMessage.error('无法创建 WebSocket 连接')
  }
}

function disconnectWs() {
  if (ws) {
    ws.close()
    ws = null
  }
  wsConnected.value = false
  subscriptions.value = []
}

function publish() {
  if (!pubTopic.value) {
    ElMessage.warning('请输入 Topic')
    return
  }
  if (!ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({
    action: 'publish',
    topic: pubTopic.value,
    payload: pubPayload.value,
    qos: pubQos.value
  }))
  messages.value.push({
    direction: 'out',
    topic: pubTopic.value,
    payload: pubPayload.value,
    qos: pubQos.value,
    time: new Date().toLocaleTimeString('zh-CN', { hour12: false })
  })
  trimMessages()
  scrollBottom()
  ElMessage.success('已发送')
}

function subscribe() {
  if (!subTopic.value) {
    ElMessage.warning('请输入订阅 Topic')
    return
  }
  if (!ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ action: 'subscribe', topic: subTopic.value }))
  subscriptions.value.push(subTopic.value)
  subTopic.value = ''
  ElMessage.success('已订阅')
}

function unsubscribe(index: number) {
  const topic = subscriptions.value[index]
  if (!ws || ws.readyState !== WebSocket.OPEN) return
  ws.send(JSON.stringify({ action: 'unsubscribe', topic }))
  subscriptions.value.splice(index, 1)
}

function trimMessages() {
  if (messages.value.length > 500) {
    messages.value = messages.value.slice(-500)
  }
}

function scrollBottom() {
  if (!autoScroll.value) return
  nextTick(() => {
    if (msgContainer.value) {
      msgContainer.value.scrollTop = msgContainer.value.scrollHeight
    }
  })
}

onMounted(() => {})
onUnmounted(() => {
  if (ws) { ws.close(); ws = null }
})
</script>

<style scoped>
.card-head { display: flex; justify-content: space-between; align-items: center; }
.ws-status { display: flex; align-items: center; gap: 6px; }
.dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; }
.dot.online { background: #67C23A; }
.dot.offline { background: #c0c4cc; }
.inner-card { border: 1px solid #ebeef5; }
.action-bar { margin-top: 12px; display: flex; justify-content: flex-end; }
.sub-add { display: flex; gap: 8px; margin-bottom: 12px; }
.sub-list { display: flex; flex-wrap: wrap; }
.msg-stream { max-height: 600px; overflow-y: auto; }
.msg-item { padding: 8px 12px; border-bottom: 1px solid #f0f0f0; }
.msg-item:last-child { border-bottom: none; }
.msg-header { display: flex; align-items: center; flex-wrap: wrap; gap: 4px; }
.msg-topic { font-family: monospace; font-size: 12px; color: #409EFF; }
.msg-payload { font-family: monospace; font-size: 13px; color: #333; margin-top: 4px; word-break: break-all; white-space: pre-wrap; }
.msg-item.in { background: #f6ffed; }
.msg-item.out { background: #fff7e6; }
</style>
