<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索产品名称" clearable style="width: 240px" @change="load">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-button type="primary" @click="$router.push('/products/new')">
        <el-icon><Plus /></el-icon>&nbsp;创建产品
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column label="产品名称" min-width="140">
        <template #default="{ row }">
          <el-link type="primary" @click="$router.push(`/products/${row.id}`)">{{ row.name }}</el-link>
        </template>
      </el-table-column>
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
      <el-table-column label="协议" width="80">
        <template #default="{ row }">{{ row.protocol.toUpperCase() }}</template>
      </el-table-column>
      <el-table-column label="密钥模式" width="90">
        <template #default="{ row }">
          <el-text size="small">{{ row.secretMode === 'product' ? '一型一密' : '一机一密' }}</el-text>
        </template>
      </el-table-column>
      <el-table-column prop="deviceCount" label="设备数" width="80" />
      <el-table-column label="创建时间" width="120">
        <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="210" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="$router.push(`/products/${row.id}`)">详情</el-button>
          <el-button link type="primary" size="small" @click="$router.push(`/products/${row.id}/edit`)">配置</el-button>
          <el-button link type="primary" size="small" @click="$router.push(`/devices?productId=${row.id}`)">设备</el-button>
          <el-popconfirm title="确定删除该产品？" @confirm="del(row)">
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
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type Product } from '../api'

const list = ref<Product[]>([])
const total = ref(0)
const page = ref(1)
const size = 10
const keyword = ref('')
const loading = ref(false)

function accessText(m: string) {
  return ({ thingmodel: '物模型', passthrough: '透传解析', modbus: 'Modbus' } as any)[m] || m
}
function accessTag(m: string) {
  return ({ thingmodel: 'success', passthrough: 'warning', modbus: 'primary' } as any)[m] || 'info'
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

async function del(row: Product) {
  await api.deleteProduct(row.id)
  ElMessage.success('已删除')
  load()
}

function copy(text: string) {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

function fmtDate(s: string) {
  return s ? new Date(s).toLocaleDateString('zh-CN') : '-'
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }
</style>
