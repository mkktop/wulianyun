<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-select v-model="typeFilter" placeholder="全部类型" clearable style="width: 160px" @change="load">
        <el-option label="阈值告警" value="alarm" />
        <el-option label="离线告警" value="offline" />
        <el-option label="数据转发" value="forward" />
      </el-select>
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon>&nbsp;创建规则
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="name" label="规则名称" min-width="130" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }">
          <el-tag :type="typeTag(row.type)">{{ typeText(row.type) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="作用范围" min-width="150">
        <template #default="{ row }">
          {{ row.productId ? row.productName : '全部产品' }}
          <span v-if="row.deviceId"> / {{ row.deviceName }}</span>
        </template>
      </el-table-column>
      <el-table-column label="条件" min-width="180">
        <template #default="{ row }">
          <el-text size="small" type="info">{{ condText(row) }}</el-text>
        </template>
      </el-table-column>
      <el-table-column label="动作" min-width="120">
        <template #default="{ row }">
          <el-tag v-for="a in actionTags(row)" :key="a" size="small" style="margin-right:4px">{{ a }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="silence" label="静默(分)" width="90" />
      <el-table-column label="启用" width="80">
        <template #default="{ row }">
          <el-switch v-model="row.enabled" @change="toggle(row)" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDialog(row)">编辑</el-button>
          <el-popconfirm title="确定删除该规则？" @confirm="del(row)">
            <template #reference><el-button link type="danger">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager" background layout="total, prev, pager, next"
      :total="total" :page-size="size" v-model:current-page="page" @current-change="load"
    />
  </el-card>

  <!-- 创建/编辑规则弹窗 -->
  <el-dialog v-model="dialogVisible" :title="editing ? '编辑规则' : '创建规则'" width="680px" @open="onDialogOpen">
    <!-- 一键模板（仅新建时显示） -->
    <div v-if="!editing" class="tpl-section">
      <div class="tpl-title">快捷模板</div>
      <div class="tpl-cards">
        <div v-for="tpl in templates" :key="tpl.key" class="tpl-card" @click="applyTemplate(tpl)">
          <div class="tpl-icon" :style="{ background: tpl.bg, color: tpl.color }">
            <el-icon :size="22"><component :is="tpl.icon" /></el-icon>
          </div>
          <div class="tpl-info">
            <div class="tpl-name">{{ tpl.name }}</div>
            <div class="tpl-desc">{{ tpl.desc }}</div>
          </div>
        </div>
      </div>
      <el-divider />
    </div>

    <el-form :model="form" label-width="100px">
      <el-form-item label="规则名称" required>
        <el-input v-model="form.name" placeholder="如：高温告警" />
      </el-form-item>
      <el-form-item label="规则类型" required>
        <el-radio-group v-model="form.type">
          <el-radio value="alarm">阈值告警</el-radio>
          <el-radio value="offline">离线告警</el-radio>
          <el-radio value="forward">数据转发</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="产品">
        <el-select v-model="form.productId" placeholder="全部产品" clearable style="width: 100%" @change="onProductChange">
          <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="设备" v-if="form.productId">
        <el-select v-model="form.deviceId" placeholder="产品下全部设备" clearable style="width: 100%">
          <el-option v-for="d in devices" :key="d.id" :label="d.name" :value="d.id" />
        </el-select>
      </el-form-item>

      <!-- 阈值告警：复合条件编辑器 -->
      <template v-if="form.type === 'alarm'">
        <el-form-item label="触发条件" required>
          <!-- 逻辑切换 -->
          <div class="cond-header">
            <el-radio-group v-model="form.logic" size="small">
              <el-radio-button value="and">满足全部 (AND)</el-radio-button>
              <el-radio-button value="or">满足任一 (OR)</el-radio-button>
            </el-radio-group>
          </div>
          <!-- 条件列表 -->
          <div class="cond-list">
            <div v-for="(c, i) in form.conditions" :key="i" class="cond-item">
              <el-input v-model="c.field" placeholder="属性，如 temperature" style="width: 160px" />
              <el-select v-model="c.op" style="width: 80px">
                <el-option v-for="o in ['>', '<', '>=', '<=', '==', '!=']" :key="o" :label="o" :value="o" />
              </el-select>
              <el-input-number v-model="c.value" :controls="false" style="width: 120px" />
              <el-button link type="danger" @click="removeCondition(i)" :disabled="form.conditions.length <= 1">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
          </div>
          <el-button link type="primary" @click="addCondition">
            <el-icon><Plus /></el-icon>&nbsp;添加条件
          </el-button>
        </el-form-item>
        <el-form-item label="告警级别">
          <el-radio-group v-model="form.level">
            <el-radio value="warning">警告</el-radio>
            <el-radio value="critical">严重</el-radio>
          </el-radio-group>
        </el-form-item>
      </template>

      <!-- 离线告警条件 -->
      <el-form-item v-if="form.type === 'offline'" label="离线超过" required>
        <el-input-number v-model="form.minutes" :min="1" :max="1440" /> &nbsp;分钟
      </el-form-item>

      <!-- 动作区：多选 -->
      <el-form-item label="触发动作">
        <el-checkbox-group v-model="form.actions">
          <el-checkbox value="alarm">站内告警</el-checkbox>
          <el-checkbox value="webhook">Webhook转发</el-checkbox>
          <el-checkbox value="kafka">Kafka</el-checkbox>
          <el-checkbox value="mqtt_bridge">MQTT桥接</el-checkbox>
        </el-checkbox-group>
      </el-form-item>

      <!-- Webhook URL -->
      <el-form-item v-if="form.actions.includes('webhook')" label="Webhook URL">
        <el-input v-model="form.webhookUrl" placeholder="http:// 或 https:// 通知地址" />
      </el-form-item>

      <!-- Kafka 配置 -->
      <template v-if="form.actions.includes('kafka')">
        <el-form-item label="Kafka Brokers">
          <el-input v-model="form.kafkaBrokers" placeholder="broker1:9092,broker2:9092" />
        </el-form-item>
        <el-form-item label="Kafka Topic">
          <el-input v-model="form.kafkaTopic" placeholder="如：iot-telemetry" />
        </el-form-item>
      </template>

      <!-- MQTT 桥接配置 -->
      <template v-if="form.actions.includes('mqtt_bridge')">
        <el-form-item label="MQTT Broker">
          <el-input v-model="form.mqttBroker" placeholder="如：tcp://broker.example.com:1883" />
        </el-form-item>
        <el-form-item label="MQTT Topic">
          <el-input v-model="form.mqttTopic" placeholder="如：/sync/data" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.mqttUsername" placeholder="可选" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.mqttPassword" type="password" placeholder="可选" show-password />
        </el-form-item>
      </template>

      <el-form-item label="静默期(分)">
        <el-input-number v-model="form.silence" :min="1" :max="1440" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type Device, type Product, type Rule } from '../api'

const list = ref<Rule[]>([])
const products = ref<Product[]>([])
const devices = ref<Device[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const typeFilter = ref('')
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref<Rule | null>(null)

interface CondItem { field: string; op: string; value: number }

const form = reactive({
  name: '', type: 'alarm', productId: undefined as number | undefined, deviceId: undefined as number | undefined,
  logic: 'and' as 'and' | 'or',
  conditions: [] as CondItem[],
  level: 'warning', minutes: 10,
  actions: ['alarm'] as string[],
  webhookUrl: '',
  kafkaBrokers: '', kafkaTopic: '',
  mqttBroker: '', mqttTopic: '', mqttUsername: '', mqttPassword: '',
  silence: 5
})

// ---- 一键模板 ----
const templates = [
  {
    key: 'high_temp', name: '高温告警', desc: '温度 > 50℃ 时触发严重告警',
    icon: 'Sunny', color: '#F56C6C', bg: '#fef0f0',
    apply: () => Object.assign(form, {
      name: '高温告警', type: 'alarm', logic: 'and',
      conditions: [{ field: 'temperature', op: '>', value: 50 }],
      level: 'critical', actions: ['alarm'], silence: 5
    })
  },
  {
    key: 'offline', name: '设备离线', desc: '设备离线超过 10 分钟告警',
    icon: 'OfflinePin', color: '#E6A23C', bg: '#fdf6ec',
    apply: () => Object.assign(form, {
      name: '设备离线告警', type: 'offline', minutes: 10, level: 'warning', actions: ['alarm'], silence: 10
    })
  },
  {
    key: 'custom', name: '自定义规则', desc: '自由配置条件，灵活转发',
    icon: 'Edit', color: '#409EFF', bg: '#ecf5ff',
    apply: () => Object.assign(form, {
      name: '', type: 'alarm', logic: 'and',
      conditions: [{ field: '', op: '>', value: 0 }],
      level: 'warning', actions: ['alarm'], silence: 5
    })
  }
]

function applyTemplate(tpl: typeof templates[number]) {
  tpl.apply()
}

function addCondition() {
  form.conditions.push({ field: '', op: '>', value: 0 })
}
function removeCondition(i: number) {
  form.conditions.splice(i, 1)
}

async function load() {
  loading.value = true
  try {
    const res = await api.listRules({ page: page.value, size, type: typeFilter.value })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function onProductChange() {
  form.deviceId = undefined
  if (form.productId) {
    const res = await api.listDevices({ page: 1, size: 100, productId: form.productId })
    devices.value = res.list
  }
}

function resetForm() {
  Object.assign(form, {
    name: '', type: 'alarm', productId: undefined, deviceId: undefined,
    logic: 'and', conditions: [{ field: '', op: '>', value: 0 }],
    level: 'warning', minutes: 10, actions: ['alarm'],
    webhookUrl: '', kafkaBrokers: '', kafkaTopic: '',
    mqttBroker: '', mqttTopic: '', mqttUsername: '', mqttPassword: '',
    silence: 5
  })
}

function parseConditionToForm(cond: any) {
  if (!cond) {
    form.conditions = [{ field: '', op: '>', value: 0 }]
    form.logic = 'and'
    return
  }
  // 复合条件格式: {logic: "and", conditions: [...]}
  if (cond.logic && Array.isArray(cond.conditions)) {
    form.logic = cond.logic
    form.conditions = cond.conditions.map((c: any) => ({
      field: c.field || '', op: c.op || '>', value: c.value ?? 0
    }))
    if (form.conditions.length === 0) form.conditions = [{ field: '', op: '>', value: 0 }]
  } else {
    // 旧格式单条件: {field, op, value}
    form.logic = 'and'
    form.conditions = [{ field: cond.field || '', op: cond.op || '>', value: cond.value ?? 0 }]
  }
}

function parseActionToForm(action: any) {
  if (!action) {
    form.actions = ['alarm']
    form.webhookUrl = ''
    return
  }
  const acts: string[] = []
  // 判断动作类型
  const actionType = action.type || ''
  if (actionType === 'kafka') acts.push('kafka')
  if (actionType === 'mqtt_bridge') acts.push('mqtt_bridge')

  const notify = action.notify || []
  if (notify.includes('ws') || action.level) acts.push('alarm')
  if (notify.includes('webhook') || action.webhookUrl) acts.push('webhook')

  form.actions = acts.length > 0 ? acts : ['alarm']
  form.webhookUrl = action.webhookUrl || ''
  form.kafkaBrokers = (action.brokers || []).join(',')
  form.kafkaTopic = action.topic || ''
  form.mqttBroker = action.broker || ''
  form.mqttTopic = action.topic || ''
  form.mqttUsername = action.username || ''
  form.mqttPassword = action.password || ''
  form.level = action.level || 'warning'
}

function buildCondition(): any {
  if (form.type === 'alarm') {
    const validConds = form.conditions.filter(c => c.field)
    if (validConds.length === 0) return {}
    if (validConds.length === 1) {
      return { field: validConds[0].field, op: validConds[0].op, value: validConds[0].value }
    }
    return {
      logic: form.logic,
      conditions: validConds.map(c => ({ field: c.field, op: c.op, value: c.value }))
    }
  }
  if (form.type === 'offline') return { minutes: form.minutes }
  return {}
}

function buildAction(): any {
  const action: any = {}
  const acts = form.actions

  if (acts.includes('alarm')) {
    action.level = form.level
    action.notify = ['ws']
  }

  if (acts.includes('webhook')) {
    action.webhookUrl = form.webhookUrl
    action.notify = [...(action.notify || []), 'webhook']
  }

  if (acts.includes('kafka')) {
    action.type = 'kafka'
    action.brokers = form.kafkaBrokers.split(',').map(s => s.trim()).filter(Boolean)
    action.topic = form.kafkaTopic
  }

  if (acts.includes('mqtt_bridge')) {
    action.type = 'mqtt_bridge'
    action.broker = form.mqttBroker
    action.topic = form.mqttTopic
    action.username = form.mqttUsername
    action.password = form.mqttPassword
  }

  return action
}

function openDialog(row?: Rule) {
  editing.value = row || null
  if (row) {
    form.name = row.name
    form.type = row.type
    form.productId = row.productId || undefined
    form.deviceId = row.deviceId || undefined
    form.silence = row.silence
    form.minutes = row.condition?.minutes || 10
    parseConditionToForm(row.condition)
    parseActionToForm(row.action)
    if (form.productId) {
      onProductChange().then(() => { form.deviceId = row.deviceId || undefined })
    }
  } else {
    resetForm()
  }
  dialogVisible.value = true
}

function onDialogOpen() {
  // 确保 ECharts / DOM 初始化完成后渲染
  nextTick()
}

async function save() {
  if (!form.name) { ElMessage.warning('请输入规则名称'); return }
  if (form.type === 'alarm' && !form.conditions.some(c => c.field)) {
    ElMessage.warning('请至少配置一个触发条件'); return
  }
  if (form.type === 'forward' && !form.actions.length) {
    ElMessage.warning('请选择至少一个转发动作'); return
  }
  if (form.actions.includes('webhook') && !form.webhookUrl) {
    ElMessage.warning('请输入 Webhook URL'); return
  }

  const condition = buildCondition()
  const action = buildAction()
  const payload = {
    name: form.name, type: form.type,
    productId: form.productId || 0, deviceId: form.deviceId || 0,
    condition, action, silence: form.silence
  }
  saving.value = true
  try {
    if (editing.value) {
      await api.updateRule(editing.value.id, payload)
    } else {
      await api.createRule(payload)
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function toggle(row: Rule) {
  await api.updateRule(row.id, {
    name: row.name, type: row.type, productId: row.productId, deviceId: row.deviceId,
    condition: row.condition, action: row.action, silence: row.silence, enabled: row.enabled
  })
  ElMessage.success(row.enabled ? '已启用' : '已停用')
}

async function del(row: Rule) {
  await api.deleteRule(row.id)
  ElMessage.success('已删除')
  load()
}

function typeText(t: string) {
  return ({ alarm: '阈值告警', offline: '离线告警', forward: '数据转发' } as any)[t] || t
}
function typeTag(t: string) {
  return ({ alarm: 'warning', offline: 'info', forward: 'success' } as any)[t] || 'info'
}
function condText(row: Rule): string {
  const c = row.condition || {}
  if (row.type === 'alarm') {
    if (c.logic && c.conditions) {
      const parts = c.conditions.map((cond: any) => `${cond.field} ${cond.op} ${cond.value}`)
      return parts.join(c.logic === 'or' ? ' 或 ' : ' 且 ')
    }
    return `${c.field || '?'} ${c.op || '?'} ${c.value ?? '?'}`
  }
  if (row.type === 'offline') return `离线超过 ${c.minutes} 分钟`
  return '-'
}

function actionTags(row: Rule): string[] {
  const a = row.action || {}
  const tags: string[] = []
  if (a.level || (a.notify || []).includes('ws')) tags.push('告警')
  if (a.webhookUrl || (a.notify || []).includes('webhook')) tags.push('Webhook')
  if (a.type === 'kafka') tags.push('Kafka')
  if (a.type === 'mqtt_bridge') tags.push('MQTT桥接')
  return tags.length ? tags : ['-']
}

onMounted(async () => {
  load()
  const res = await api.listProducts({ page: 1, size: 100 })
  products.value = res.list
})
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }

/* 条件编辑器 */
.cond-header { margin-bottom: 12px; }
.cond-list { display: flex; flex-direction: column; gap: 8px; margin-bottom: 8px; }
.cond-item { display: flex; align-items: center; gap: 8px; }

/* 模板卡片 */
.tpl-section { margin-bottom: 4px; }
.tpl-title { font-weight: 600; font-size: 14px; color: #303133; margin-bottom: 12px; }
.tpl-cards { display: flex; gap: 12px; }
.tpl-card {
  flex: 1; display: flex; align-items: center; gap: 12px; padding: 14px;
  border: 1px solid #e4e7ed; border-radius: 8px; cursor: pointer; transition: all 0.2s;
}
.tpl-card:hover { border-color: #409EFF; box-shadow: 0 2px 8px rgba(64,158,255,0.12); }
.tpl-icon {
  width: 42px; height: 42px; border-radius: 10px;
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.tpl-info { min-width: 0; }
.tpl-name { font-weight: 600; font-size: 13px; color: #303133; }
.tpl-desc { font-size: 12px; color: #909399; margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
