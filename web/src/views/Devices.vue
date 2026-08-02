<template>
  <el-card shadow="never">
    <div class="toolbar">
      <div class="filters">
        <el-select v-model="productId" placeholder="全部产品" clearable style="width: 200px" @change="load">
          <el-option v-for="p in products" :key="p.id" :label="p.name" :value="String(p.id)" />
        </el-select>
        <el-select v-model="status" placeholder="全部状态" clearable style="width: 140px" @change="load">
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
          <el-option label="未激活" value="inactive" />
          <el-option label="已禁用" value="disabled" />
        </el-select>
        <el-select v-model="groupId" placeholder="全部分组" clearable style="width: 150px" @change="load">
          <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="String(g.id)" />
        </el-select>
        <el-input v-model="keyword" placeholder="搜索设备名称" clearable style="width: 200px" @change="load">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>
      <div>
        <el-button @click="groupMgrVisible = true">分组管理</el-button>
        <el-button type="primary" @click="dialogVisible = true">
          <el-icon><Plus /></el-icon>&nbsp;添加设备
        </el-button>
      </div>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column label="设备名称" min-width="140">
        <template #default="{ row }">
          <el-link type="primary" @click="$router.push(`/devices/${row.id}`)">{{ row.name }}</el-link>
        </template>
      </el-table-column>
      <el-table-column prop="productName" label="所属产品" min-width="120" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="分组" width="110">
        <template #default="{ row }">{{ row.groupName || '-' }}</template>
      </el-table-column>
      <el-table-column label="标签" min-width="120">
        <template #default="{ row }">
          <el-tag v-for="t in parseTags(row.tags)" :key="t" size="small" style="margin-right: 4px">{{ t }}</el-tag>
          <span v-if="!parseTags(row.tags).length" class="muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="最后上线" width="160">
        <template #default="{ row }">{{ fmt(row.lastOnlineAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="$router.push(`/devices/${row.id}`)">详情</el-button>
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button link type="warning" size="small" @click="toggleDisable(row)">
            {{ row.status === 'disabled' ? '启用' : '禁用' }}
          </el-button>
          <el-popconfirm title="确定删除该设备？" @confirm="del(row)">
            <template #reference><el-button link type="danger" size="small">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="pager" background layout="total, prev, pager, next"
      :total="total" :page-size="size" v-model:current-page="page" @current-change="load"
    />
  </el-card>

  <el-dialog v-model="dialogVisible" title="添加设备" width="480px">
    <el-form :model="form" label-width="90px">
      <el-form-item label="所属产品" required>
        <el-select v-model="form.productId" placeholder="选择产品" style="width: 100%">
          <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="设备名称" required>
        <el-input v-model="form.name" placeholder="产品内唯一，如 sensor-001" />
      </el-form-item>
      <el-form-item label="注册码" v-if="isTcpProduct(form.productId)">
        <el-input v-model="form.regCode" placeholder="选填，IMEI/ICCID 等，TCP 设备可凭注册码免三元组接入" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.remark" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">确定</el-button>
    </template>
  </el-dialog>

  <!-- 编辑设备：分组/标签/备注 -->
  <el-dialog v-model="editVisible" title="编辑设备" width="460px">
    <el-form label-width="80px">
      <el-form-item label="设备名称">
        <el-text>{{ editing?.name }}</el-text>
      </el-form-item>
      <el-form-item label="分组">
        <el-select v-model="editForm.groupId" clearable placeholder="未分组" style="width: 100%">
          <el-option :value="0" label="未分组" />
          <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="标签">
        <div class="tag-edit">
          <el-tag v-for="(t, i) in editForm.tags" :key="t" closable @close="editForm.tags.splice(i, 1)">{{ t }}</el-tag>
          <el-input v-model="newTag" size="small" style="width: 100px" placeholder="+标签" @keyup.enter="addTag" @blur="addTag" />
        </div>
      </el-form-item>
      <el-form-item label="注册码" v-if="isTcpProduct(editing?.productId)">
        <el-input v-model="editForm.regCode" placeholder="选填，IMEI/ICCID 等" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="editForm.remark" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="saveEdit">保存</el-button>
    </template>
  </el-dialog>

  <!-- 分组管理 -->
  <el-dialog v-model="groupMgrVisible" title="分组管理" width="520px">
    <div class="group-add">
      <el-input v-model="newGroupName" placeholder="新分组名称" style="flex: 1" />
      <el-button type="primary" @click="addGroup">添加</el-button>
    </div>
    <el-table :data="groups" size="small" max-height="320">
      <el-table-column prop="name" label="分组名称" min-width="140" />
      <el-table-column prop="deviceCount" label="设备数" width="80" />
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-popconfirm title="删除分组？组内设备将置为未分组" @confirm="removeGroup(row)">
            <template #reference><el-button link type="danger" size="small">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api, type Device, type DeviceGroup, type Product } from '../api'
import { realtime } from '../utils/realtime'

const route = useRoute()
const list = ref<Device[]>([])
const products = ref<Product[]>([])
const groups = ref<DeviceGroup[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const keyword = ref('')
const status = ref('')
const productId = ref((route.query.productId as string) || '')
const groupId = ref('')
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const form = reactive<{ productId: number | null; name: string; remark: string; regCode: string }>({
  productId: null, name: '', remark: '', regCode: ''
})

// 编辑设备
const editVisible = ref(false)
const editing = ref<Device | null>(null)
const editForm = reactive<{ groupId: number; tags: string[]; remark: string; regCode: string }>({ groupId: 0, tags: [], remark: '', regCode: '' })
const newTag = ref('')

// 是否 TCP 接入产品（注册码仅对 TCP 设备有意义）
function isTcpProduct(pid?: number | null) {
  if (!pid) return false
  return products.value.find((p) => p.id === pid)?.protocol === 'tcp'
}

// 分组管理
const groupMgrVisible = ref(false)
const newGroupName = ref('')

function parseTags(v: any): string[] {
  if (!v) return []
  if (Array.isArray(v)) return v
  try { return JSON.parse(v) } catch { return [] }
}

function openEdit(row: Device) {
  editing.value = row
  editForm.groupId = row.groupId || 0
  editForm.tags = [...parseTags(row.tags)]
  editForm.remark = row.remark
  editForm.regCode = row.regCode || ''
  newTag.value = ''
  editVisible.value = true
}

function addTag() {
  const t = newTag.value.trim()
  if (t && !editForm.tags.includes(t)) editForm.tags.push(t)
  newTag.value = ''
}

async function saveEdit() {
  saving.value = true
  try {
    await api.updateDevice(editing.value!.id, {
      remark: editForm.remark, groupId: editForm.groupId, tags: editForm.tags, regCode: editForm.regCode
    })
    ElMessage.success('已保存')
    editVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function loadGroups() {
  groups.value = await api.listGroups()
}

async function addGroup() {
  if (!newGroupName.value.trim()) return
  await api.createGroup({ name: newGroupName.value.trim() })
  newGroupName.value = ''
  loadGroups()
}

async function removeGroup(g: DeviceGroup) {
  await api.deleteGroup(g.id)
  ElMessage.success('已删除')
  loadGroups()
  load()
}

async function load() {
  loading.value = true
  try {
    const res = await api.listDevices({
      page: page.value, size, keyword: keyword.value,
      status: status.value, productId: productId.value, groupId: groupId.value
    })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function loadProducts() {
  const res = await api.listProducts({ page: 1, size: 100 })
  products.value = res.list
}

async function save() {
  if (!form.productId || !form.name) {
    ElMessage.warning('请选择产品并输入设备名称')
    return
  }
  saving.value = true
  try {
    await api.createDevice(form)
    ElMessage.success('设备创建成功')
    dialogVisible.value = false
    form.name = ''
    form.remark = ''
    form.regCode = ''
    load()
  } finally {
    saving.value = false
  }
}

async function toggleDisable(row: Device) {
  await api.updateDevice(row.id, { remark: row.remark, disable: row.status !== 'disabled' })
  ElMessage.success(row.status === 'disabled' ? '已启用' : '已禁用')
  load()
}

async function del(row: Device) {
  await api.deleteDevice(row.id)
  ElMessage.success('已删除')
  load()
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

// 设备状态实时刷新
function onMsg(msg: any) {
  if (msg.type === 'device_status') {
    const d = list.value.find((x) => x.id === msg.deviceId)
    if (d) d.status = msg.payload.status
  }
}

onMounted(() => {
  load()
  loadProducts()
  loadGroups()
  realtime.on(onMsg)
})
onUnmounted(() => realtime.off(onMsg))
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.filters { display: flex; gap: 12px; }
.pager { margin-top: 16px; justify-content: flex-end; }
.tag-edit { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
.group-add { display: flex; gap: 8px; margin-bottom: 12px; }
.muted { color: #c0c4cc; }
</style>
