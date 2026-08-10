<template>
  <el-card shadow="never">
    <div class="toolbar">
      <div class="filters">
        <el-input v-model="filters.keyword" placeholder="用户名 / 昵称" clearable style="width: 180px" @keyup.enter="load" />
        <el-select v-model="filters.role" placeholder="角色" clearable style="width: 130px" @change="load">
          <el-option label="超管" value="admin" />
          <el-option label="普通用户" value="user" />
        </el-select>
        <el-select v-model="filters.status" placeholder="状态" clearable style="width: 130px" @change="load">
          <el-option label="启用" value="active" />
          <el-option label="禁用" value="disabled" />
        </el-select>
        <el-button type="primary" @click="load">查询</el-button>
      </div>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>&nbsp;新建用户
      </el-button>
    </div>

    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="username" label="用户名" min-width="130" />
      <el-table-column prop="nickname" label="昵称" min-width="120" />
      <el-table-column label="层级" width="100">
        <template #default="{ row }">
          <el-tag :type="row.tier === 'platform' ? 'danger' : row.tier === 'primary' ? 'primary' : 'info'" size="small">
            {{ row.tier === 'platform' ? '超管' : row.tier === 'primary' ? '一级' : '二级' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="parentName" label="上级账号" width="110">
        <template #default="{ row }">{{ row.parentName || '-' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'danger'" size="small">
            {{ row.status === 'active' ? '启用' : '禁用' }}
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
      <el-table-column prop="deviceCount" label="设备数" width="80" />
      <el-table-column label="创建时间" width="110">
        <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="250" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" size="small" @click="resetPwd(row)">重置密码</el-button>
          <el-button
            v-if="row.id !== myId"
            link :type="row.status === 'active' ? 'warning' : 'success'" size="small" @click="toggleStatus(row)"
          >
            {{ row.status === 'active' ? '禁用' : '启用' }}
          </el-button>
          <el-popconfirm v-if="row.id !== myId" title="确定删除该用户？（需名下无设备与子账号）" @confirm="del(row)">
            <template #reference><el-button link type="danger" size="small">删除</el-button></template>
          </el-popconfirm>
          <span v-else class="self-hint">当前账号</span>
        </template>
      </el-table-column>
    </el-table>
    <div style="margin-top: 12px; display: flex; justify-content: flex-end">
      <el-pagination
        v-model:current-page="page"
        :page-size="size"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="load"
      />
    </div>

    <!-- 新建用户 -->
    <el-dialog v-model="createVisible" title="新建用户" width="460px" :close-on-click-modal="false">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="80px" @submit.prevent>
        <el-form-item label="用户名" prop="username">
          <el-input v-model="createForm.username" placeholder="登录用户名（≥3位）" />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="createForm.nickname" placeholder="如：运营团队" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="createForm.password" type="password" show-password placeholder="初始密码（≥6位）" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="createForm.role" style="width: 100%">
            <el-option label="普通用户（一级账号）" value="user" />
            <el-option label="平台超管（拥有系统管理权限）" value="admin" />
          </el-select>
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

    <!-- 编辑用户 -->
    <el-dialog v-model="editVisible" title="编辑用户" width="460px">
      <el-form label-width="80px" @submit.prevent>
        <el-form-item label="用户名"><el-input :model-value="editForm.username" disabled /></el-form-item>
        <el-form-item label="昵称"><el-input v-model="editForm.nickname" /></el-form-item>
        <el-form-item label="角色">
          <el-select v-model="editForm.role" :disabled="editForm.id === myId" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="平台超管" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item label="权限">
          <el-select v-model="editForm.permission" style="width: 100%">
            <el-option label="可操作" value="operate" />
            <el-option label="只读" value="view" />
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
import { api, type AdminUser } from '../../api'
import { fmtDate } from '../../utils/format'

const myId = Number(localStorage.getItem('userId') || 0)
const list = ref<AdminUser[]>([])
const loading = ref(false)
const page = ref(1)
const size = 10
const total = ref(0)
const filters = reactive({ keyword: '', role: '', status: '' })

const createVisible = ref(false)
const creating = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({ username: '', nickname: '', password: '', role: 'user', permission: 'operate' })
const createRules: FormRules = {
  username: [{ required: true, min: 3, max: 32, message: '用户名 3-32 位', trigger: 'blur' }],
  password: [{ required: true, min: 6, max: 64, message: '密码 6-64 位', trigger: 'blur' }],
}

const editVisible = ref(false)
const editSaving = ref(false)
const editForm = reactive({ id: 0, username: '', nickname: '', role: 'user', permission: 'operate' })

async function load() {
  loading.value = true
  try {
    const data = await api.admin.users.list({ page: page.value, size: size.value, ...filters })
    list.value = data.list || []
    total.value = data.total || 0
  } catch { /* 错误已由拦截器提示 */ } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(createForm, { username: '', nickname: '', password: '', role: 'user', permission: 'operate' })
  createVisible.value = true
}

async function doCreate() {
  if (!createFormRef.value) return
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return
    creating.value = true
    try {
      await api.admin.users.create({ ...createForm })
      ElMessage.success('创建成功')
      createVisible.value = false
      load()
    } catch { /* 错误已由拦截器提示 */ } finally {
      creating.value = false
    }
  })
}

function openEdit(row: AdminUser) {
  Object.assign(editForm, { id: row.id, username: row.username, nickname: row.nickname, role: row.role, permission: row.permission })
  editVisible.value = true
}

async function doEdit() {
  editSaving.value = true
  try {
    await api.admin.users.update(editForm.id, {
      nickname: editForm.nickname, role: editForm.role, permission: editForm.permission,
    })
    ElMessage.success('已保存')
    editVisible.value = false
    load()
  } catch { /* 错误已由拦截器提示 */ } finally {
    editSaving.value = false
  }
}

async function resetPwd(row: AdminUser) {
  const { value } = await ElMessageBox.prompt(`为用户 ${row.username} 设置新密码`, '重置密码', {
    inputType: 'password', inputPlaceholder: '新密码（≥6位）',
    inputValidator: (v: string) => (v && v.length >= 6 ? true : '密码至少 6 位'),
  })
  await api.admin.users.update(row.id, { password: value })
  ElMessage.success('密码已重置')
}

async function toggleStatus(row: AdminUser) {
  const disable = row.status === 'active'
  if (disable && row.id === myId) {
    ElMessage.warning('不能禁用自己的账号')
    return
  }
  try {
    await api.admin.users.update(row.id, { status: disable ? 'disabled' : 'active' })
    ElMessage.success(disable ? '已禁用' : '已启用')
    load()
  } catch { /* 错误已由拦截器提示 */ }
}

async function del(row: AdminUser) {
  try {
    await api.admin.users.remove(row.id)
    ElMessage.success('已删除')
    load()
  } catch { /* 错误已由拦截器提示 */ }
}

onMounted(load)
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.filters { display: flex; gap: 10px; }
.self-hint { color: #c0c4cc; font-size: 12px; }
</style>
