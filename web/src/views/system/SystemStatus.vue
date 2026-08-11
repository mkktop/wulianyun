<template>
  <div class="system-page">
    <SysPageHeader title="系统状态" desc="服务进程、各组件健康与全局统计" icon="Odometer">
      <el-button type="primary" :loading="loading" @click="load">
        <el-icon><Refresh /></el-icon>&nbsp;刷新
      </el-button>
    </SysPageHeader>

    <!-- 组件健康 -->
    <el-row :gutter="16">
      <el-col v-for="item in healthItems" :key="item.label" :span="6">
        <div :class="['sp-health', item.ok ? 'ok' : 'bad']">
          <div class="sp-health-icon">
            <el-icon><component :is="item.icon" /></el-icon>
          </div>
          <div>
            <div class="sp-health-label">{{ item.label }}</div>
            <div class="sp-health-value">{{ item.text }}</div>
            <div v-if="item.error" class="sp-health-error">{{ item.error }}</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 全局统计（渐变图标方块卡片） -->
    <div style="margin-top: 16px"></div>
    <div class="sp-section-title">全局统计</div>
    <el-row :gutter="16">
      <el-col v-for="s in stats" :key="s.label" :span="4">
        <div class="sp-stat">
          <div :class="['sp-stat-icon', s.color]">
            <el-icon><component :is="s.icon" /></el-icon>
          </div>
          <div>
            <div class="sp-stat-value">{{ s.value }}</div>
            <div class="sp-stat-label">{{ s.label }}</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 服务进程信息（图标网格替代表格） -->
    <div class="sp-card" style="margin-top: 16px">
      <div class="sp-card-header"><span><el-icon><Monitor /></el-icon>服务进程</span></div>
      <div class="sp-card-body">
        <div class="sp-grid">
          <div class="sp-grid-item">
            <div class="sp-grid-label">服务版本</div>
            <div class="sp-grid-value">{{ status.version || '-' }}</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">运行时长</div>
            <div class="sp-grid-value">{{ fmtDuration(status.uptimeSeconds) }}</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">Go 版本</div>
            <div class="sp-grid-value">{{ status.goVersion || '-' }}</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">Goroutines</div>
            <div class="sp-grid-value">{{ status.goroutines ?? '-' }}</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">内存占用</div>
            <div class="sp-grid-value">{{ status.memAllocMB ?? '-' }} MB</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">内存总额</div>
            <div class="sp-grid-value">{{ status.memSysMB ?? '-' }} MB</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">TCP 网关连接</div>
            <div class="sp-grid-value">{{ status.gateway?.tcpConnections ?? '-' }}</div>
          </div>
          <div class="sp-grid-item">
            <div class="sp-grid-label">EMQX 规则引擎</div>
            <div class="sp-grid-value">
              <el-tag :type="status.emqxRule ? 'success' : 'info'" size="small">
                {{ status.emqxRule ? '已启用' : '未启用' }}
              </el-tag>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../../api'
import SysPageHeader from '../../components/SysPageHeader.vue'

const status = ref<any>({})
const loading = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

// 健康卡片（图标 + 状态）
const healthItems = computed(() => {
  const s = status.value || {}
  const mk = (label: string, icon: string, ok: boolean, text: string, error = '') =>
    ({ label, icon, ok, text, error })
  return [
    mk('数据库', 'Coin', s.db?.status === 'ok', s.db?.status === 'ok' ? '正常' : '异常', s.db?.error),
    mk('Redis 缓存', 'Histogram', s.redis?.status === 'ok', s.redis?.status === 'ok' ? '正常' : '异常', s.redis?.error),
    mk('MQTT Broker', 'Connection', !!s.mqtt?.connected, s.mqtt?.connected ? '已连接' : '未连接'),
    mk('TCP 网关', 'Position', (s.gateway?.tcpConnections ?? 0) >= 0, `${s.gateway?.tcpConnections ?? 0} 个连接`),
  ]
})

// 全局统计卡片（渐变图标 + 数据）
const stats = computed(() => {
  const c = status.value?.counts || {}
  return [
    { label: '用户总数', value: c.users ?? '-', icon: 'User', color: 'purple' },
    { label: '产品总数', value: c.products ?? '-', icon: 'Box', color: 'blue' },
    { label: '设备总数', value: c.devices ?? '-', icon: 'Cpu', color: 'green' },
    { label: '在线设备', value: c.online ?? '-', icon: 'Connection', color: 'cyan' },
    { label: '今日消息', value: c.msgToday ?? '-', icon: 'ChatDotSquare', color: 'orange' },
    { label: '消息总量', value: c.msgTotal ?? '-', icon: 'DataAnalysis', color: 'red' },
  ]
})

function fmtDuration(sec: number | undefined): string {
  if (sec == null) return '-'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return d > 0 ? `${d}天${h}小时` : h > 0 ? `${h}小时${m}分` : m > 0 ? `${m}分${s}秒` : `${s}秒`
}

async function load() {
  loading.value = true
  try {
    status.value = await api.admin.systemStatus()
  } catch { /* 错误已由拦截器提示 */ } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  timer = setInterval(load, 30000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>
