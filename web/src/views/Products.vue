<template>
  <el-card shadow="never">
    <div class="toolbar">
      <el-input v-model="keyword" placeholder="搜索产品名称" clearable style="width: 240px" @change="load">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
      <el-button v-if="!secondary" type="primary" @click="$router.push('/products/new')">
        <el-icon><Plus /></el-icon>&nbsp;创建产品
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <template #empty>
        <el-empty :description="secondary ? '暂无可用的下放产品，请联系主账号下放' : '暂无产品，点击右上角创建'" :image-size="80" />
      </template>
      <el-table-column label="产品名称" min-width="140">
        <template #default="{ row }">
          <el-link type="primary" @click="$router.push(`/products/${row.id}`)">{{ row.name }}</el-link>
        </template>
      </el-table-column>
      <el-table-column label="ProductKey" min-width="200">
        <template #default="{ row }">
          <el-text type="info">{{ row.productId }}</el-text>
          <el-button link type="primary" size="small" @click="copyText(row.productId)">复制</el-button>
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
      <el-table-column label="操作" width="140" fixed="right" class-name="col-ops">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="$router.push(`/products/${row.id}`)">详情</el-button>
          <el-dropdown trigger="click" @command="(c: string) => onCmd(c, row)">
            <el-button link type="primary" size="small">
              更多<el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="!secondary" command="edit">配置</el-dropdown-item>
                <el-dropdown-item command="devices">设备列表</el-dropdown-item>
                <el-dropdown-item v-if="!secondary" command="del" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
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
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api, type Product, isSecondary } from '../api'
import { fmtDate } from '../utils/format'
import { copyText } from '../utils/clipboard'

const router = useRouter()

const secondary = isSecondary()
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

function onCmd(cmd: string, row: Product) {
  if (cmd === 'edit') router.push(`/products/${row.id}/edit`)
  else if (cmd === 'devices') router.push(`/devices?productId=${row.id}`)
  else if (cmd === 'del') {
    ElMessageBox.confirm(`确定删除产品「${row.name}」？`, '删除确认', { type: 'warning' })
      .then(() => del(row)).catch(() => {})
  }
}


onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 16px; }
.pager { margin-top: 16px; justify-content: flex-end; }
:deep(.col-ops .cell) {
  white-space: nowrap;
  display: flex; align-items: center; justify-content: center; gap: 4px;
}
:deep(.col-ops .cell .el-button) { margin: 0; }
</style>
