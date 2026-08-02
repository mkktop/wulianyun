<template>
  <div>
    <el-tabs v-model="activeTab">
      <el-tab-pane label="固件管理" name="firmware">
        <el-card shadow="never">
          <div class="toolbar">
            <el-select v-model="fwProductFilter" placeholder="全部产品" clearable style="width: 200px" @change="loadFirmwares">
              <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
            </el-select>
            <el-button type="primary" @click="showUpload = true">
              <el-icon><Upload /></el-icon>&nbsp;上传新固件
            </el-button>
          </div>
          <el-table :data="firmwares" v-loading="fwLoading" stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column label="产品" min-width="140">
              <template #default="{ row }">{{ productName(row.productId) }}</template>
            </el-table-column>
            <el-table-column prop="version" label="版本" width="120" />
            <el-table-column label="大小" width="110">
              <template #default="{ row }">{{ formatSize(row.fileSize) }}</template>
            </el-table-column>
            <el-table-column label="校验值(SHA-256)" width="140" show-overflow-tooltip>
              <template #default="{ row }">
                <el-text size="small" type="info">{{ row.checksum ? row.checksum.substring(0, 16) + '...' : '-' }}</el-text>
              </template>
            </el-table-column>
            <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
            <el-table-column label="上传时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="{ row }">
                <el-button link type="danger" @click="deleteFw(row.id)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="升级任务" name="tasks">
        <el-card shadow="never">
          <div class="toolbar">
            <el-button type="primary" @click="showCreateTask = true">
              <el-icon><Promotion /></el-icon>&nbsp;创建升级任务
            </el-button>
            <el-button @click="loadTasks" :icon="Refresh">刷新</el-button>
          </div>
          <el-table :data="tasks" v-loading="taskLoading" stripe>
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column label="产品" min-width="120">
              <template #default="{ row }">{{ row.productName || '-' }}</template>
            </el-table-column>
            <el-table-column label="固件版本" width="110">
              <template #default="{ row }">{{ row.firmwareVersion || '-' }}</template>
            </el-table-column>
            <el-table-column label="设备数" width="80">
              <template #default="{ row }">{{ row.deviceCount || 0 }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="进度" width="140">
              <template #default="{ row }">
                <el-progress :percentage="row.progress" :status="row.status === 'completed' ? 'success' : row.status === 'failed' ? 'exception' : ''" />
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="170">
              <template #default="{ row }">{{ fmt(row.createdAt) }}</template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 上传固件对话框 -->
    <el-dialog v-model="showUpload" title="上传固件" width="520px">
      <el-form :model="uploadForm" label-width="90px">
        <el-form-item label="产品" required>
          <el-select v-model="uploadForm.productId" placeholder="选择产品" style="width: 100%" filterable>
            <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="版本号" required>
          <el-input v-model="uploadForm.version" placeholder="如 1.0.0" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="uploadForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="固件文件" required>
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            :on-change="onFileChange"
            :on-remove="onFileRemove"
            :drag="true"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">拖拽文件到此处或 <em>点击上传</em></div>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUpload = false">取消</el-button>
        <el-button type="primary" :loading="uploading" @click="submitUpload">上传</el-button>
      </template>
    </el-dialog>

    <!-- 创建升级任务对话框 -->
    <el-dialog v-model="showCreateTask" title="创建升级任务" width="560px">
      <el-form :model="taskForm" label-width="90px">
        <el-form-item label="产品" required>
          <el-select v-model="taskForm.productId" placeholder="选择产品（筛选固件和设备）" style="width: 100%" filterable @change="onTaskProductChange">
            <el-option v-for="p in products" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="固件" required>
          <el-select v-model="taskForm.firmwareId" placeholder="选择固件版本" style="width: 100%">
            <el-option v-for="fw in filteredFirmwares" :key="fw.id" :label="`${fw.version} (ID:${fw.id})`" :value="fw.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="升级设备" required>
          <el-select
            v-model="taskForm.deviceIds"
            multiple
            filterable
            placeholder="选择需要升级的设备"
            style="width: 100%"
          >
            <el-option v-for="d in taskDevices" :key="d.id" :label="d.name" :value="d.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateTask = false">取消</el-button>
        <el-button type="primary" :loading="creatingTask" @click="submitTask">创建并下发</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { api, type Product, type Device } from '../api'

const activeTab = ref('firmware')

// 产品列表（供选择器使用）
const products = ref<Product[]>([])

// 固件列表
const firmwares = ref<any[]>([])
const fwLoading = ref(false)
const fwProductFilter = ref<number | undefined>(undefined)
const fwProductNames = ref<Record<number, string>>({})

// 升级任务列表
const tasks = ref<any[]>([])
const taskLoading = ref(false)

// 上传对话框
const showUpload = ref(false)
const uploading = ref(false)
const uploadForm = ref({ productId: undefined as number | undefined, version: '', description: '' })
const uploadRef = ref()
let selectedFile: File | null = null

// 创建任务对话框
const showCreateTask = ref(false)
const creatingTask = ref(false)
const taskForm = ref({
  productId: undefined as number | undefined,
  firmwareId: undefined as number | undefined,
  deviceIds: [] as number[]
})
const taskDevices = ref<Device[]>([])

// 按产品过滤固件
const filteredFirmwares = computed(() => {
  if (!taskForm.value.productId) return firmwares.value
  return firmwares.value.filter(fw => fw.productId === taskForm.value.productId)
})

function productName(pid: number): string {
  return fwProductNames.value[pid] || `#${pid}`
}

async function loadProducts() {
  const res = await api.listProducts({ page: 1, size: 200 })
  products.value = res.list
}

async function loadFirmwares() {
  fwLoading.value = true
  try {
    const res = await api.ota.firmwares(fwProductFilter.value ? { productId: fwProductFilter.value } : undefined) as any
    firmwares.value = res.list || res
    fwProductNames.value = res.productNames || {}
  } finally {
    fwLoading.value = false
  }
}

async function loadTasks() {
  taskLoading.value = true
  try {
    tasks.value = (await api.ota.tasks()) as any[]
  } finally {
    taskLoading.value = false
  }
}

function onFileChange(file: any) {
  selectedFile = file.raw
}
function onFileRemove() {
  selectedFile = null
}

async function submitUpload() {
  if (!uploadForm.value.productId || !uploadForm.value.version) {
    ElMessage.warning('请填写产品和版本号')
    return
  }
  if (!selectedFile) {
    ElMessage.warning('请选择固件文件')
    return
  }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('productId', String(uploadForm.value.productId))
    fd.append('version', uploadForm.value.version)
    fd.append('description', uploadForm.value.description)
    fd.append('file', selectedFile)
    await api.ota.uploadFirmware(fd)
    ElMessage.success('固件上传成功')
    showUpload.value = false
    uploadForm.value = { productId: undefined, version: '', description: '' }
    selectedFile = null
    loadFirmwares()
  } finally {
    uploading.value = false
  }
}

async function deleteFw(id: number) {
  await ElMessageBox.confirm('确认删除该固件？关联的物理文件也会被删除。', '提示')
  await api.ota.deleteFirmware(id)
  ElMessage.success('已删除')
  loadFirmwares()
}

async function onTaskProductChange() {
  taskForm.value.firmwareId = undefined
  taskForm.value.deviceIds = []
  if (taskForm.value.productId) {
    const res = await api.listDevices({ page: 1, size: 500, productId: taskForm.value.productId })
    taskDevices.value = res.list
  } else {
    taskDevices.value = []
  }
}

async function submitTask() {
  if (!taskForm.value.firmwareId || taskForm.value.deviceIds.length === 0) {
    ElMessage.warning('请选择固件和升级设备')
    return
  }
  creatingTask.value = true
  try {
    const res = await api.ota.createTask({
      firmwareId: taskForm.value.firmwareId,
      deviceIds: taskForm.value.deviceIds
    }) as any
    const pushed = res?.pushedCount ?? 0
    const total = res?.totalDevices ?? taskForm.value.deviceIds.length
    ElMessage.success(`任务已创建，成功下发 ${pushed}/${total} 台设备`)
    showCreateTask.value = false
    taskForm.value = { productId: undefined, firmwareId: undefined, deviceIds: [] }
    taskDevices.value = []
    loadTasks()
  } finally {
    creatingTask.value = false
  }
}

function formatSize(bytes: number) {
  if (!bytes) return '-'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

function statusType(s: string) {
  return ({ pending: 'info', running: '', completed: 'success', failed: 'danger' } as any)[s] || 'info'
}

function statusText(s: string) {
  return ({ pending: '待执行', running: '执行中', completed: '已完成', failed: '失败' } as any)[s] || s
}

function fmt(s: string) {
  return s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
}

onMounted(async () => {
  await loadProducts()
  loadFirmwares()
  loadTasks()
})
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; margin-bottom: 12px; }
</style>
