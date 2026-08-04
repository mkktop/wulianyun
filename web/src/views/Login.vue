<template>
  <div class="login-page">
    <div class="deco-grid"></div>
    <div class="deco-glow g1"></div>
    <div class="deco-glow g2"></div>
    <div class="deco-dot d1"></div>
    <div class="deco-dot d2"></div>
    <div class="deco-dot d3"></div>
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
import { api, computeTier } from '../api'

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
      localStorage.setItem('tier', computeTier(user))
      localStorage.setItem('perm', user.permission || 'operate')
      router.push('/')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  position: relative; overflow: hidden;
  height: 100%; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #0f2027, #203a43, #2c5364);
}
/* 科技感网格背景 */
.deco-grid {
  position: absolute; inset: 0;
  background-image:
    linear-gradient(rgba(126, 231, 255, 0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(126, 231, 255, 0.06) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse at center, black 30%, transparent 75%);
  -webkit-mask-image: radial-gradient(ellipse at center, black 30%, transparent 75%);
}
/* 光晕 */
.deco-glow {
  position: absolute; border-radius: 50%; filter: blur(90px); opacity: .35;
  animation: float 8s ease-in-out infinite alternate;
}
.g1 { width: 420px; height: 420px; left: -120px; top: -100px; background: #1668dc; }
.g2 { width: 380px; height: 380px; right: -100px; bottom: -120px; background: #0e7c86; animation-delay: -4s; }
/* 浮动光点 */
.deco-dot {
  position: absolute; border-radius: 50%; background: #7ee7ff;
  box-shadow: 0 0 12px 2px rgba(126, 231, 255, .8);
  animation: float 6s ease-in-out infinite alternate;
}
.d1 { width: 8px; height: 8px; left: 22%; top: 30%; }
.d2 { width: 6px; height: 6px; right: 24%; top: 22%; animation-delay: -2s; }
.d3 { width: 10px; height: 10px; right: 16%; bottom: 28%; animation-delay: -4s; }
@keyframes float {
  from { transform: translateY(-12px); }
  to { transform: translateY(12px); }
}
/* 无障碍：用户偏好减少动效时停止装饰动画 */
@media (prefers-reduced-motion: reduce) {
  .deco-glow, .deco-dot { animation: none; }
}
.login-card {
  position: relative; z-index: 1;
  width: 400px; padding: 12px 8px; border-radius: 14px;
  border: none; box-shadow: 0 16px 48px rgba(0, 0, 0, .35);
  backdrop-filter: blur(6px);
}
.brand { text-align: center; margin-bottom: 24px; }
.brand h2 { margin: 8px 0 4px; }
.brand p { color: #999; font-size: 13px; }
.submit { width: 100%; margin-top: 4px; }
.switch { text-align: center; margin-top: 12px; }
</style>
