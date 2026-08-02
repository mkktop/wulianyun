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
              <el-dropdown-item command="logout">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElNotification } from 'element-plus'
import { realtime } from '../utils/realtime'

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

onMounted(() => realtime.on(onMsg))
onUnmounted(() => realtime.off(onMsg))

function onCommand(cmd: string) {
  if (cmd === 'logout') {
    realtime.close()
    localStorage.removeItem('token')
    localStorage.removeItem('username')
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
.header {
  display: flex; align-items: center; justify-content: space-between;
  background: #fff; border-bottom: 1px solid #e8e8e8;
}
.title { font-size: 16px; font-weight: 600; }
.user { display: flex; align-items: center; gap: 4px; cursor: pointer; color: #333; }
.main { background: #f0f2f5; }
</style>
