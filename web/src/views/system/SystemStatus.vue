<template>
  <el-card shadow="never">
    <div class="toolbar">
      <span class="desc">平台超管专属：查看服务进程、各组件健康状态与全局统计（admin 视角，不过滤账号）</span>
      <el-button type="primary" :loading="loading" @click="load">
        <el-icon><Refresh /></el-icon>&nbsp;刷新
      </el-button>
    </div>

    <!-- 组件健康 -->
    <el-row :gutter="12" class="status-row">
      <el-col v-for="item in healthItems" :key="item.label" :span="6">
        <el-card shadow="hover" :class="['health-card', item.ok ? 'ok' : 'bad']">
          <div class="health-head">
            <span class="health-dot" :class="item.ok ? 'dot-ok' : 'dot-bad'"></span>
            <span class="health-label">{{ item.label }}</span>
          </div>
          <div class="health-value">{{ item.text }}</div>
          <div v-if="item.error" class="health-error">{{ item.error }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 进程信息 -->
    <el-descriptions title="服务进程" :column="4" border class="mt-16">
      <el-descriptions-item label="服务版本">{{ status.version || '-' }}</el-descriptions-item>
      <el-descriptions-item label="运行时长">{{ fmtDuration(status.uptimeSeconds) }}</el-descriptions-item>
      <el-descriptions-item label="Go 版本">{{ status.goVersion || '-' }}</el-descriptions-item>
      <el-descriptions-item label="Goroutines">{{ status.goroutines ?? '-' }}</el-descriptions-item>
      <el-descriptions-item label="内存占用">{{ status.memAllocMB ?? '-' }} MB</el-descriptions-item>
      <el-descriptions-item label="内存总额">{{ status.memSysMB ?? '-' }} MB</el-descriptions-item>
      <el-descriptions-item label="TCP 网关连接">{{ status.gateway?.tcpConnections ?? '-' }}</el-descriptions-item>
      <el-descriptions-item label="EMQX 规则引擎">
        <el-tag :type="status.emqxRule ? 'success' : 'info'" size="small">
          {{ status.emqxRule ? '已启用' : '未启用' }}
        </el-tag>
      </el-descriptions-item>
    </el-descriptions>

    <!-- 全局统计 -->
    <el-descriptions title="全局统计" :column="5" border class="mt-16">
      <el-descriptions-item label="用户总数">{{ status.counts?.users ?? '-' }}</el-descriptions-item>
      <el-descriptions-item label="产品总数">{{ status.counts?.products ?? '-' }}</el-descriptions-item>
      <el-descriptions-item label="设备总数">{{ status.counts?.devices ?? '-' }}</el-descriptions-item>
      <el-descriptions-item label="在线设备">{{ status.counts?.online ?? '-' }}</el-descriptions-item>
      <el-descriptions-item label="今日消息量">{{ status.counts?.msgToday ?? '-' }}</el-descriptions-item>
    </el-descriptions>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '../../api'

const status = ref<any>({})
const loading = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

// 组件健康卡片（ok/error/未连接 三种态）
const healthItems = computed(() => {
  const s = status.value || {}
  const mk = (label: string, ok: boolean, text: string, error = '') => ({ label, ok, text, error })
  return [
    mk('数据库', s.db?.status === 'ok', s.db?.status === 'ok' ? '正常' : '异常', s.db?.error),
    mk('Redis', s.redis?.status === 'ok', s.redis?.status === 'ok' ? '正常' : '异常', s.redis?.error),
    mk('MQTT Broker', !!s.mqtt?.connected, s.mqtt?.connected ? '已连接' : '未连接'),
    mk('TCP 网关', (s.gateway?.tcpConnections ?? 0) >= 0, `${s.gateway?.tcpConnections ?? 0} 个连接`),
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
  timer = setInterval(load, 30000) // 30s 自动刷新
})
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.desc { color: #909399; font-size: 13px; }
.status-row { margin-bottom: 16px; }
.health-card { text-align: center; }
.health-head { display: flex; align-items: center; justify-content: center; gap: 6px; margin-bottom: 8px; }
.health-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.dot-ok { background: #67c23a; box-shadow: 0 0 6px rgba(103, 194, 58, 0.6); }
.dot-bad { background: #f56c6c; box-shadow: 0 0 6px rgba(245, 108, 108, 0.6); }
.health-card .health-label { color: #909399; font-size: 13px; }
.health-card .health-value { font-size: 18px; font-weight: 600; }
.health-card.ok .health-value { color: #67c23a; }
.health-card.bad .health-value { color: #f56c6c; }
.health-error { color: #f56c6c; font-size: 12px; margin-top: 6px; word-break: break-all; }
.mt-16 { margin-top: 16px; }
</style>
