<template>
  <div class="login-page">
    <el-card class="login-card">
      <div class="brand">
        <el-icon :size="36" color="#409EFF"><Platform /></el-icon>
        <h2>KK物联云</h2>
        <p>设备接入 · 数据管理 · 实时监控</p>
      </div>
      <el-form :model="form" @keyup.enter="submit">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" size="large">
            <template #prefix><el-icon><User /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" size="large" show-password>
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item v-if="isRegister">
          <el-input v-model="form.nickname" placeholder="昵称（选填）" size="large">
            <template #prefix><el-icon><Postcard /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-button type="primary" size="large" class="submit" :loading="loading" @click="submit">
          {{ isRegister ? '注册' : '登录' }}
        </el-button>
        <div class="switch">
          <el-link type="primary" @click="isRegister = !isRegister">
            {{ isRegister ? '已有账号？去登录' : '没有账号？注册一个' }}
          </el-link>
        </div>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const router = useRouter()
const isRegister = ref(false)
const loading = ref(false)
const form = reactive({ username: '', password: '', nickname: '' })

async function submit() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    if (isRegister.value) {
      await api.register(form)
      ElMessage.success('注册成功，请登录')
      isRegister.value = false
    } else {
      const { token, user } = await api.login(form)
      localStorage.setItem('token', token)
      localStorage.setItem('username', user.nickname || user.username)
      router.push('/')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  height: 100%; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #0f2027, #203a43, #2c5364);
}
.login-card { width: 400px; padding: 12px 8px; border-radius: 10px; }
.brand { text-align: center; margin-bottom: 24px; }
.brand h2 { margin: 8px 0 4px; }
.brand p { color: #999; font-size: 13px; }
.submit { width: 100%; margin-top: 4px; }
.switch { text-align: center; margin-top: 12px; }
</style>
