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

  <el-dialog v-model="dialogVisible" :title="editing ? '编辑规则' : '创建规则'" width="560px">
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

      <!-- 阈值告警条件 -->
      <template v-if="form.type === 'alarm'">
        <el-form-item label="触发条件" required>
          <div class="cond-row">
            <el-input v-model="form.field" placeholder="属性，如 temperature" style="width: 180px" />
            <el-select v-model="form.op" style="width: 90px">
              <el-option v-for="o in ['>', '<', '>=', '<=', '==', '!=']" :key="o" :label="o" :value="o" />
            </el-select>
            <el-input-number v-model="form.value" :controls="false" style="width: 120px" />
          </div>
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

      <!-- Webhook（转发必填，告警可选） -->
      <el-form-item :label="form.type === 'forward' ? '转发地址' : 'Webhook'" :required="form.type === 'forward'">
        <el-input v-model="form.webhookUrl" placeholder="http:// 或 https:// 通知地址（可选）" />
      </el-form-item>

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
import { onMounted, reactive, ref } from 'vue'
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

const form = reactive({
  name: '', type: 'alarm', productId: undefined as number | undefined, deviceId: undefined as number | undefined,
  field: '', op: '>', value: 0, level: 'warning', minutes: 10, webhookUrl: '', silence: 5
})

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

function openDialog(row?: Rule) {
  editing.value = row || null
  if (row) {
    form.name = row.name
    form.type = row.type
    form.productId = row.productId || undefined
    form.deviceId = row.deviceId || undefined
    form.silence = row.silence
    const c = row.condition || {}
    const a = row.action || {}
    form.field = c.field || ''
    form.op = c.op || '>'
    form.value = c.value ?? 0
    form.minutes = c.minutes || 10
    form.level = a.level || 'warning'
    form.webhookUrl = a.webhookUrl || ''
    if (form.productId) onProductChange().then(() => { form.deviceId = row.deviceId || undefined })
  } else {
    Object.assign(form, {
      name: '', type: 'alarm', productId: undefined, deviceId: undefined,
      field: '', op: '>', value: 0, level: 'warning', minutes: 10, webhookUrl: '', silence: 5
    })
  }
  dialogVisible.value = true
}

async function save() {
  if (!form.name) { ElMessage.warning('请输入规则名称'); return }
  if (form.type === 'alarm' && !form.field) { ElMessage.warning('请输入触发条件属性'); return }
  if (form.type === 'forward' && !form.webhookUrl) { ElMessage.warning('请输入转发地址'); return }

  let condition: any = {}
  let action: any = {}
  if (form.type === 'alarm') {
    condition = { field: form.field, op: form.op, value: form.value }
    action = { level: form.level, notify: form.webhookUrl ? ['ws', 'webhook'] : ['ws'], webhookUrl: form.webhookUrl }
  } else if (form.type === 'offline') {
    condition = { minutes: form.minutes }
    action = { level: form.level, notify: form.webhookUrl ? ['ws', 'webhook'] : ['ws'], webhookUrl: form.webhookUrl }
  } else {
    action = { webhookUrl: form.webhookUrl }
  }
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
function condText(row: Rule) {
  const c = row.condition || {}
  if (row.type === 'alarm') return `${c.field} ${c.op} ${c.value}`
  if (row.type === 'offline') return `离线超过 ${c.minutes} 分钟`
  return (row.action || {}).webhookUrl || '-'
}

onMounted(async () => {
  load()
  const res = await api.listProducts({ page: 1, size: 100 })
  products.value = res.list
})
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.cond-row { display: flex; gap: 8px; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
