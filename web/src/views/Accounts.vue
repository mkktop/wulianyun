<template>
  <el-card shadow="never">
    <div class="toolbar">
      <span class="desc">管理名下二级账号，下放产品供其使用（二级仅能使用被下放的产品类型，无法创建产品）</span>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>&nbsp;新建子账号
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column prop="nickname" label="昵称" min-width="120" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
            {{ row.status === 'active' ? '启用' : '已禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="权限" width="90">
        <template #default="{ row }">
          <el-tag :type="row.permission === 'view' ? 'info' : 'success'" size="small">
            {{ row.permission === 'view' ? '只读' : '可操作' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="deviceCount" label="设备数" width="90" />
      <el-table-column prop="grantCount" label="已下放产品" width="110" />
      <el-table-column label="创建时间" width="120">
        <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="230" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" size="small" @click="resetPwd(row)">重置密码</el-button>
          <el-button link :type="row.status === 'active' ? 'warning' : 'success'" size="small" @click="toggleStatus(row)">
            {{ row.status === 'active' ? '禁用' : '启用' }}
          </el-button>
          <el-popconfirm title="确定删除该子账号？（需先清空其设备）" @confirm="del(row)">
            <template #reference><el-button link type="danger" size="small">删除</el-button></template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新建子账号 -->
    <el-dialog v-model="createVisible" title="新建子账号" width="440px" :close-on-click-modal="false">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="80px" @submit.prevent>
        <el-form-item label="用户名" prop="username">
          <el-input v-model="createForm.username" placeholder="登录用户名（≥3位）" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="createForm.nickname" placeholder="如：华东分公司" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="createForm.password" type="password" show-password placeholder="初始密码（≥6位）" />
        </el-form-item>
        <el-form-item label="权限" prop="permission">
          <el-select v-model="createForm.permission" style="width: 100%">
            <el-option label="可操作（管理设备/下发指令）" value="operate" />
            <el-option label="只读（仅查看）" value="view" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="doCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- 编辑昵称 -->
    <el-dialog v-model="editVisible" title="编辑子账号" width="440px">
      <el-form label-width="80px">
        <el-form-item label="用户名"><el-input :model-value="editForm.username" disabled /></el-form-item>
        <el-form-item label="昵称"><el-input v-model="editForm.nickname" /></el-form-item>
        <el-form-item label="权限">
          <el-select v-model="editForm.permission" style="width: 100%">
            <el-option label="可操作（管理设备/下发指令）" value="operate" />
            <el-option label="只读（仅查看）" value="view" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="editSaving" @click="doEdit">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { api, type Account } from '../api'
import { fmtDate } from '../utils/format'

const list = ref<Account[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    list.value = await api.listAccounts()
  } finally {
    loading.value = false
  }
}

// 创建
const createVisible = ref(false)
const creating = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({ username: '', nickname: '', password: '', permission: 'operate' })
const createRules: FormRules = {
  username: [{ required: true, min: 3, max: 32, message: '用户名 3-32 位', trigger: 'blur' }],
  password: [{ required: true, min: 6, max: 64, message: '密码至少 6 位', trigger: 'blur' }],
}
function openCreate() {
  createForm.username = ''
  createForm.nickname = ''
  createForm.password = ''
  createForm.permission = 'operate'
  createVisible.value = true
}
async function doCreate() {
  if (!createFormRef.value) return
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return
    creating.value = true
    try {
      await api.createAccount({ ...createForm })
      ElMessage.success('子账号已创建')
      createVisible.value = false
      load()
    } finally {
      creating.value = false
    }
  })
}

// 编辑昵称
const editVisible = ref(false)
const editSaving = ref(false)
const editForm = reactive({ id: 0, username: '', nickname: '', permission: 'operate' })
function openEdit(row: Account) {
  editForm.id = row.id
  editForm.username = row.username
  editForm.nickname = row.nickname
  editForm.permission = row.permission || 'operate'
  editVisible.value = true
}
async function doEdit() {
  editSaving.value = true
  try {
    await api.updateAccount(editForm.id, { nickname: editForm.nickname, permission: editForm.permission })
    ElMessage.success('已保存')
    editVisible.value = false
    load()
  } finally {
    editSaving.value = false
  }
}

// 重置密码
async function resetPwd(row: Account) {
  try {
    const { value } = await ElMessageBox.prompt('请输入新密码（6-64 位）', `重置 ${row.username} 的密码`, {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      inputPattern: /^.{6,64}$/,
      inputErrorMessage: '密码长度 6-64 位',
    })
    await api.updateAccount(row.id, { password: value })
    ElMessage.success('密码已重置')
  } catch { /* 取消 */ }
}

// 启用/禁用
async function toggleStatus(row: Account) {
  const next = row.status === 'active' ? 'disabled' : 'active'
  await api.updateAccount(row.id, { status: next })
  ElMessage.success(next === 'active' ? '已启用' : '已禁用')
  load()
}

async function del(row: Account) {
  await api.deleteAccount(row.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.desc { color: #999; font-size: 13px; }
</style>
