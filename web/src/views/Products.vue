<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索产品名称" clearable style="width: 240px" @change="load">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon>&nbsp;创建产品
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="name" label="产品名称" min-width="140" />
      <el-table-column label="ProductKey" min-width="200">
        <template #default="{ row }">
          <el-text type="info">{{ row.productKey }}</el-text>
          <el-button link type="primary" size="small" @click="copy(row.productKey)">复制</el-button>
        </template>
      </el-table-column>
      <el-table-column label="接入方式" width="110">
        <template #default="{ row }">
          <el-tag :type="accessTag(row.accessMode)" size="small">{{ accessText(row.accessMode) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="密钥模式" width="90">
        <template #default="{ row }">
          <el-text size="small">{{ row.secretMode === 'product' ? '一型一密' : '一机一密' }}</el-text>
        </template>
      </el-table-column>
      <el-table-column prop="deviceCount" label="设备数" width="80" />
      <el-table-column label="创建时间" width="160">
        <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }">
          <el-button v-if="row.accessMode !== 'passthrough'" link type="primary" @click="openTsl(row)">物模型</el-button>
          <el-button v-if="row.accessMode === 'passthrough'" link type="primary" @click="openCodec(row)">解析脚本</el-button>
          <el-button v-if="row.accessMode === 'modbus'" link type="primary" @click="openModbus(row)">点位表</el-button>
          <el-button link type="primary" @click="$router.push(`/devices?productId=${row.id}`)">设备</el-button>
          <el-button link type="primary" @click="openDialog(row)">编辑</el-button>
          <el-popconfirm title="确定删除该产品？" @confirm="del(row)">
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

  <el-dialog v-model="dialogVisible" :title="editing ? '编辑产品' : '创建产品'" width="520px">
    <el-form :model="form" label-width="90px">
      <el-form-item label="产品名称" required>
        <el-input v-model="form.name" placeholder="如：温湿度传感器" />
      </el-form-item>
      <el-form-item label="接入方式">
        <el-radio-group v-model="form.accessMode" :disabled="!!editing">
          <el-radio value="thingmodel">物模型</el-radio>
          <el-radio value="passthrough">透传解析</el-radio>
          <el-radio value="modbus">Modbus</el-radio>
        </el-radio-group>
        <div class="hint">{{ accessHint }}</div>
      </el-form-item>
      <el-form-item label="接入协议" v-if="form.accessMode !== 'modbus'">
        <el-radio-group v-model="form.protocol" :disabled="!!editing">
          <el-radio value="mqtt">MQTT</el-radio>
          <el-radio value="http">HTTP</el-radio>
          <el-radio value="tcp">TCP透传</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="采集周期" v-if="form.accessMode === 'modbus'">
        <el-input-number v-model="form.pollInterval" :min="60" :step="60" :disabled="!!editing" /> &nbsp;秒（最小 60）
      </el-form-item>
      <el-form-item label="密钥模式">
        <el-radio-group v-model="form.secretMode" :disabled="!!editing">
          <el-radio value="device">一机一密</el-radio>
          <el-radio value="product">一型一密</el-radio>
        </el-radio-group>
        <div class="hint">{{ form.secretMode === 'product' ? '产品共用密钥，设备首次连接自动注册' : '每个设备独立密钥，安全性高' }}</div>
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="2" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">确定</el-button>
    </template>
  </el-dialog>

  <!-- 一型一密创建成功：展示产品密钥 -->
  <el-dialog v-model="secretDialogVisible" title="产品创建成功" width="460px">
    <el-alert type="success" :closable="false" style="margin-bottom: 12px">请妥善保存产品密钥，用于设备一型一密接入</el-alert>
    <el-descriptions :column="1" border>
      <el-descriptions-item label="ProductKey">{{ createdProduct?.productKey }}</el-descriptions-item>
      <el-descriptions-item label="ProductSecret">
        <el-text>{{ createdProduct?.productSecret }}</el-text>
        <el-button link type="primary" size="small" @click="copy(createdProduct?.productSecret || '')">复制</el-button>
      </el-descriptions-item>
    </el-descriptions>
    <template #footer>
      <el-button type="primary" @click="secretDialogVisible = false">我已保存</el-button>
    </template>
  </el-dialog>

  <ThingModelDialog v-model:visible="tslVisible" :product-id="tslProductId" />
  <CodecDialog v-model:visible="codecVisible" :product-id="codecProductId" />
  <ModbusPointDialog v-model:visible="modbusVisible" :product-id="modbusProductId" />
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type Product } from '../api'
import ThingModelDialog from '../components/ThingModelDialog.vue'
import CodecDialog from '../components/CodecDialog.vue'
import ModbusPointDialog from '../components/ModbusPointDialog.vue'

const list = ref<Product[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const keyword = ref('')
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref<Product | null>(null)
const form = reactive({
  name: '', protocol: 'mqtt', accessMode: 'thingmodel',
  secretMode: 'device', pollInterval: 60, description: ''
})
const tslVisible = ref(false)
const tslProductId = ref<number | null>(null)
const codecVisible = ref(false)
const codecProductId = ref<number | null>(null)
const modbusVisible = ref(false)
const modbusProductId = ref<number | null>(null)
const secretDialogVisible = ref(false)
const createdProduct = ref<Product | null>(null)

const accessHint = computed(() => ({
  thingmodel: '设备上报标准 JSON，按物模型属性解析',
  passthrough: '设备上报自定义报文，用 JS 脚本解析',
  modbus: 'DTU 主动接入，平台按采集周期轮询 Modbus 点位'
} as any)[form.accessMode])

function accessText(m: string) {
  return ({ thingmodel: '物模型', passthrough: '透传解析', modbus: 'Modbus' } as any)[m] || m
}
function accessTag(m: string) {
  return ({ thingmodel: 'success', passthrough: 'warning', modbus: 'primary' } as any)[m] || 'info'
}

function openTsl(row: Product) {
  tslProductId.value = row.id
  tslVisible.value = true
}

function openCodec(row: Product) {
  codecProductId.value = row.id
  codecVisible.value = true
}

function openModbus(row: Product) {
  modbusProductId.value = row.id
  modbusVisible.value = true
}

async function load() {
  loading.value = true
  try {
    const res = await api.listProducts({ page: page.value, size, keyword: keyword.value })
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

function openDialog(row?: Product) {
  editing.value = row || null
  form.name = row?.name || ''
  form.protocol = row?.protocol || 'mqtt'
  form.accessMode = row?.accessMode || 'thingmodel'
  form.secretMode = row?.secretMode || 'device'
  form.pollInterval = row?.pollInterval || 60
  form.description = row?.description || ''
  dialogVisible.value = true
}

async function save() {
  if (!form.name) {
    ElMessage.warning('请输入产品名称')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await api.updateProduct(editing.value.id, form)
      ElMessage.success('保存成功')
    } else {
      const p = await api.createProduct(form)
      // 一型一密：弹窗展示产品密钥
      if (p.secretMode === 'product' && p.productSecret) {
        createdProduct.value = p
        secretDialogVisible.value = true
      } else {
        ElMessage.success('创建成功')
      }
    }
    dialogVisible.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function del(row: Product) {
  await api.deleteProduct(row.id)
  ElMessage.success('已删除')
  load()
}

function copy(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

function fmt(s: string) {
  return s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }
.hint { color: #999; font-size: 12px; margin-top: 4px; line-height: 1.4; }
</style>
