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
        <el-input v-model="keyword" placeholder="搜索设备名称" clearable style="width: 200px" @change="load">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
      </div>
      <el-button type="primary" @click="dialogVisible = true">
        <el-icon><Plus /></el-icon>&nbsp;添加设备
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column label="设备名称" min-width="140">
        <template #default="{ row }">
          <el-link type="primary" @click="$router.push(`/devices/${row.id}`)">{{ row.name }}</el-link>
        </template>
      </el-table-column>
      <el-table-column prop="productName" label="所属产品" min-width="130" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最后上线" width="170">
        <template #default="{ row }">{{ fmt(row.lastOnlineAt) }}</template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="130" show-overflow-tooltip />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="$router.push(`/devices/${row.id}`)">详情</el-button>
          <el-button link type="warning" @click="toggleDisable(row)">
            {{ row.status === 'disabled' ? '启用' : '禁用' }}
          </el-button>
          <el-popconfirm title="确定删除该设备？" @confirm="del(row)">
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
      <el-form-item label="备注">
        <el-input v-model="form.remark" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api, type Device, type Product } from '../api'
import { realtime } from '../utils/realtime'

const route = useRoute()
const list = ref<Device[]>([])
const products = ref<Product[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const keyword = ref('')
const status = ref('')
const productId = ref((route.query.productId as string) || '')
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const form = reactive<{ productId: number | null; name: string; remark: string }>({
  productId: null, name: '', remark: ''
})

async function load() {
  loading.value = true
  try {
    const res = await api.listDevices({
      page: page.value, size, keyword: keyword.value,
      status: status.value, productId: productId.value
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
  realtime.on(onMsg)
})
onUnmounted(() => realtime.off(onMsg))
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.filters { display: flex; gap: 12px; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
