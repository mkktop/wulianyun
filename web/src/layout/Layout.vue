<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <el-icon :size="26" color="#409EFF"><Platform /></el-icon>
        <span>KK物联云</span>
      </div>
      <el-menu :default-active="active" router background-color="#001529" text-color="#a6adb4" active-text-color="#fff">
        <el-menu-item index="/overview">
          <el-icon><Odometer /></el-icon><span>平台概览</span>
        </el-menu-item>
        <el-menu-item index="/products">
          <el-icon><Box /></el-icon><span>产品管理</span>
        </el-menu-item>
        <el-menu-item index="/devices">
          <el-icon><Cpu /></el-icon><span>设备管理</span>
        </el-menu-item>
        <el-menu-item index="/rules">
          <el-icon><SetUp /></el-icon><span>规则引擎</span>
        </el-menu-item>
        <el-menu-item index="/alarms">
          <el-icon><BellFilled /></el-icon><span>告警中心</span>
        </el-menu-item>
        <el-menu-item index="/apps">
          <el-icon><Key /></el-icon><span>应用管理</span>
        </el-menu-item>
        <el-menu-item v-if="tier === 'primary'" index="/accounts">
          <el-icon><UserFilled /></el-icon><span>子账号管理</span>
        </el-menu-item>
        <el-menu-item index="/ota">
          <el-icon><UploadFilled /></el-icon><span>OTA升级</span>
        </el-menu-item>
        <el-menu-item index="/screen">
          <el-icon><DataBoard /></el-icon><span>可视化大屏</span>
        </el-menu-item>
        <el-sub-menu index="tools">
          <template #title><el-icon><Monitor /></el-icon><span>开发工具</span></template>
          <el-menu-item index="/tools/simulator">设备模拟器</el-menu-item>
          <el-menu-item index="/tools/mqtt-debug">MQTT调试台</el-menu-item>
          <el-menu-item index="/tools/traces">消息轨迹</el-menu-item>
        </el-sub-menu>
      </el-menu>
      <a class="docs-link" href="/developer/" target="_blank" rel="noopener">
        <el-icon><Document /></el-icon><span>开发文档</span>
        <el-icon class="external"><Right /></el-icon>
      </a>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span class="title">{{ route.meta.title || '' }}</span>
        <el-dropdown @command="onCommand">
          <span class="user">
            <el-icon><User /></el-icon>
            {{ username }}
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="password">修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>

    <el-dialog v-model="pwdVisible" title="修改密码" width="420px" :close-on-click-modal="false">
      <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-width="80px" @submit.prevent>
        <el-form-item label="原密码" prop="oldPassword">
          <el-input v-model="pwdForm.oldPassword" type="password" show-password placeholder="请输入原密码" />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="pwdForm.newPassword" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="pwdForm.confirmPassword" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="submitPassword">确认修改</el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElNotification, type FormInstance, type FormRules } from 'element-plus'
import { realtime } from '../utils/realtime'
import { api } from '../api'

const route = useRoute()
const router = useRouter()
const active = computed(() => (route.path.startsWith('/devices') ? '/devices' : route.path))
const username = localStorage.getItem('username') || '用户'

// 全局告警弹窗
function onMsg(msg: any) {
  if (msg.type === 'alarm') {
    ElNotification({
      title: `告警：${msg.payload.ruleName}`,
      message: msg.payload.message,
      type: msg.payload.level === 'critical' ? 'error' : 'warning',
      duration: 8000,
      onClick: () => router.push('/alarms')
    })
  }
}

const tier = ref(localStorage.getItem('tier') || 'primary')

onMounted(async () => {
  realtime.on(onMsg)
  // 兼容旧会话：tier 缺失则拉 profile 补全
  if (!localStorage.getItem('tier')) {
    try {
      const p = await api.profile()
      tier.value = p.tier || 'primary'
      localStorage.setItem('tier', tier.value)
    } catch { /* ignore */ }
  }
})
onUnmounted(() => realtime.off(onMsg))

// ---- 修改密码 ----
const pwdVisible = ref(false)
const pwdLoading = ref(false)
const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
const pwdRules: FormRules = {
  oldPassword: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 64, message: '密码长度 6-64 位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (_r: any, value: string, cb: (e?: Error) => void) =>
        value === pwdForm.newPassword ? cb() : cb(new Error('两次输入的密码不一致')),
      trigger: 'blur'
    }
  ]
}

function openPassword() {
  pwdForm.oldPassword = ''
  pwdForm.newPassword = ''
  pwdForm.confirmPassword = ''
  pwdVisible.value = true
}

async function submitPassword() {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate(async (valid) => {
    if (!valid) return
    pwdLoading.value = true
    try {
      await api.changePassword({ oldPassword: pwdForm.oldPassword, newPassword: pwdForm.newPassword })
      ElMessage.success('密码修改成功')
      pwdVisible.value = false
    } catch {
      /* 错误已由 axios 拦截器提示 */
    } finally {
      pwdLoading.value = false
    }
  })
}

function onCommand(cmd: string) {
  if (cmd === 'password') {
    openPassword()
  } else if (cmd === 'logout') {
    realtime.close()
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    localStorage.removeItem('tier')
    router.push('/login')
  }
}
</script>

<style scoped>
.layout { height: 100%; }
.aside { background: #001529; }
.logo {
  height: 60px; display: flex; align-items: center; justify-content: center; gap: 8px;
  color: #fff; font-size: 18px; font-weight: 600;
}
.aside :deep(.el-menu) { border-right: none; }
.docs-link {
  display: flex; align-items: center; gap: 6px;
  margin: 8px 12px; padding: 0 20px; height: 40px;
  color: #a6adb4; font-size: 14px; text-decoration: none;
  border-radius: 4px; transition: background .2s, color .2s;
}
.docs-link:hover { background: #1f3a5f; color: #fff; }
.docs-link .external { margin-left: auto; font-size: 12px; opacity: .6; }
.header {
  display: flex; align-items: center; justify-content: space-between;
  background: #fff; border-bottom: 1px solid #e8e8e8;
}
.title { font-size: 16px; font-weight: 600; }
.user { display: flex; align-items: center; gap: 4px; cursor: pointer; color: #333; }
.main { background: #f0f2f5; }
</style>
