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
      <el-table-column prop="protocol" label="接入协议" width="100">
        <template #default="{ row }">
          <el-tag>{{ row.protocol.toUpperCase() }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="deviceCount" label="设备数" width="90" />
      <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip />
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="310" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openTsl(row)">物模型</el-button>
          <el-button link type="primary" @click="openCodec(row)">解析脚本</el-button>
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

  <el-dialog v-model="dialogVisible" :title="editing ? '编辑产品' : '创建产品'" width="480px">
    <el-form :model="form" label-width="90px">
      <el-form-item label="产品名称" required>
        <el-input v-model="form.name" placeholder="如：温湿度传感器" />
      </el-form-item>
      <el-form-item label="接入协议">
        <el-radio-group v-model="form.protocol" :disabled="!!editing">
          <el-radio value="mqtt">MQTT</el-radio>
          <el-radio value="http">HTTP</el-radio>
          <el-radio value="tcp">TCP透传</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="描述">
        <el-input v-model="form.description" type="textarea" :rows="3" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">确定</el-button>
    </template>
  </el-dialog>

  <ThingModelDialog v-model:visible="tslVisible" :product-id="tslProductId" />
  <CodecDialog v-model:visible="codecVisible" :product-id="codecProductId" />
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type Product } from '../api'
import ThingModelDialog from '../components/ThingModelDialog.vue'
import CodecDialog from '../components/CodecDialog.vue'

const list = ref<Product[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const keyword = ref('')
const loading = ref(false)
const dialogVisible = ref(false)
const saving = ref(false)
const editing = ref<Product | null>(null)
const form = reactive({ name: '', protocol: 'mqtt', description: '' })
const tslVisible = ref(false)
const tslProductId = ref<number | null>(null)
const codecVisible = ref(false)
const codecProductId = ref<number | null>(null)

function openTsl(row: Product) {
  tslProductId.value = row.id
  tslVisible.value = true
}

function openCodec(row: Product) {
  codecProductId.value = row.id
  codecVisible.value = true
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
    } else {
      await api.createProduct(form)
    }
    ElMessage.success('保存成功')
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
</style>
