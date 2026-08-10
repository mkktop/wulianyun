<template>
  <el-container class="layout">
    <el-aside :width="collapsed ? '64px' : '220px'" class="aside">
      <div class="logo">
        <el-icon :size="26" color="#409EFF"><Platform /></el-icon>
        <span v-if="!collapsed">KK物联云</span>
      </div>
      <el-menu :default-active="active" router :collapse="collapsed" :collapse-transition="false" background-color="#001529" text-color="#a6adb4" active-text-color="#fff">
        <el-menu-item index="/console/overview">
          <el-icon><Odometer /></el-icon><template #title><span>平台概览</span></template>
        </el-menu-item>
        <el-menu-item index="/console/products">
          <el-icon><Box /></el-icon><template #title><span>产品管理</span></template>
        </el-menu-item>
        <el-menu-item index="/console/devices">
          <el-icon><Cpu /></el-icon><template #title><span>设备管理</span></template>
        </el-menu-item>
        <el-menu-item index="/console/rules">
          <el-icon><SetUp /></el-icon><template #title><span>规则引擎</span></template>
        </el-menu-item>
        <el-menu-item index="/console/alarms">
          <el-icon><BellFilled /></el-icon><template #title><span>告警中心</span></template>
        </el-menu-item>
        <el-menu-item index="/console/apps">
          <el-icon><Key /></el-icon><template #title><span>应用管理</span></template>
        </el-menu-item>
        <el-menu-item v-if="tier !== 'secondary'" index="/console/accounts">
          <el-icon><UserFilled /></el-icon><template #title><span>子账号管理</span></template>
        </el-menu-item>
        <el-menu-item index="/console/ota">
          <el-icon><UploadFilled /></el-icon><template #title><span>OTA升级</span></template>
        </el-menu-item>
        <el-menu-item index="/screen">
          <el-icon><DataBoard /></el-icon><template #title><span>可视化大屏</span></template>
        </el-menu-item>
        <!-- 平台超管专属后台（仅 admin 角色可见；后端 AdminAuth 兜底拦截） -->
        <el-sub-menu v-if="tier === 'platform'" index="system">
          <template #title><el-icon><Setting /></el-icon><span>系统管理</span></template>
          <el-menu-item index="/console/system/status">系统状态</el-menu-item>
          <el-menu-item index="/console/system/config">参数配置</el-menu-item>
          <el-menu-item index="/console/system/announcements">公告管理</el-menu-item>
          <el-menu-item index="/console/system/help-docs">帮助中心</el-menu-item>
          <el-menu-item index="/console/system/users">用户管理</el-menu-item>
          <el-menu-item index="/console/system/logs">全局日志</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="tools">
          <template #title><el-icon><Monitor /></el-icon><span>开发工具</span></template>
          <el-menu-item index="/console/tools/simulator">设备模拟器</el-menu-item>
          <el-menu-item index="/console/tools/mqtt-debug">MQTT调试台</el-menu-item>
          <el-menu-item index="/console/tools/traces">消息轨迹</el-menu-item>
        </el-sub-menu>
      </el-menu>
      <a class="docs-link" href="/developer/" target="_blank" rel="noopener" :title="'开发文档'">
        <el-icon><Document /></el-icon><span v-if="!collapsed">开发文档</span>
        <el-icon v-if="!collapsed" class="external"><Right /></el-icon>
      </a>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-icon class="collapse-btn" :size="18" @click="collapsed = !collapsed">
            <Expand v-if="collapsed" /><Fold v-else />
          </el-icon>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item v-for="(c, i) in breadcrumbs" :key="i" :to="c.path ? { path: c.path } : undefined">
              {{ c.title }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <Clock />
          <!-- 公告铃铛：有新发布公告时显示红点，点击查看全部 -->
          <el-badge :value="unreadCount" :max="99" :hidden="unreadCount === 0" class="ann-badge">
            <el-icon :size="18" class="bell" @click="openAnnouncements"><Bell /></el-icon>
          </el-badge>
          <el-dropdown @command="onCommand">
            <span class="user">
              <span class="avatar">{{ username.slice(0, 1).toUpperCase() }}</span>
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
        </div>
      </el-header>
      <el-main class="main">
        <el-alert v-if="perm === 'view'" type="warning" :closable="false" style="margin-bottom: 12px">
          当前为只读账号：仅可查看数据，无法执行新增、修改、删除、下发等操作
        </el-alert>
        <router-view v-slot="{ Component, route }">
          <transition name="page-fade">
            <component :is="Component" :key="route.path" />
          </transition>
        </router-view>
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

    <!-- 平台公告（已发布，所有登录账号可见） -->
    <el-dialog v-model="annVisible" title="平台公告" width="640px">
      <div v-if="announcements.length === 0" class="ann-empty">暂无公告</div>
      <div
        v-for="a in announcements" :key="a.id"
        class="ann-item" :class="{ important: a.level === 'important', expanded: expandedAnn === a.id }"
      >
        <div class="ann-row" @click="toggleAnn(a.id)">
          <span class="ann-title">{{ a.title }}</span>
          <el-tag v-if="a.level === 'important'" size="small" type="danger">重要</el-tag>
          <span class="ann-meta">{{ a.publisher }} · {{ formatTime(a.publishAt) }}</span>
          <el-icon class="ann-arrow"><component :is="expandedAnn === a.id ? 'ArrowUp' : 'ArrowDown'" /></el-icon>
        </div>
        <div v-if="expandedAnn === a.id && a.content" class="ann-content" v-html="md(a.content)"></div>
      </div>
    </el-dialog>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElNotification, type FormInstance, type FormRules } from 'element-plus'
import { marked } from 'marked'
import { realtime } from '../utils/realtime'
import { debounce } from '../utils/debounce'
import { api, type Announcement } from '../api'
import Clock from '../components/Clock.vue'

const route = useRoute()
const router = useRouter()
const active = computed(() => {
  // 控制台整体挂载在 /console 前缀下，菜单 index 为 /console/*，剥掉前缀后做详情页归组匹配
  const p = route.path.startsWith('/console') ? route.path.slice('/console'.length) : route.path
  return '/console' + (p.startsWith('/devices') ? '/devices' : p.startsWith('/products') ? '/products' : p)
})
const username = localStorage.getItem('username') || '用户'
const collapsed = ref(false)

// 面包屑：详情页回链上级列表（控制台路径统一带 /console 前缀）
const breadcrumbs = computed(() => {
  const crumbs: { title: string; path?: string }[] = []
  const p = route.path.replace(/^\/console/, '')
  if (p.startsWith('/products/') || p.startsWith('/devices/')) {
    crumbs.push(p.startsWith('/products')
      ? { title: '产品管理', path: '/console/products' }
      : { title: '设备管理', path: '/console/devices' })
  }
  const t = route.meta.title as string
  if (t) crumbs.push({ title: t })
  return crumbs
})

// 全局告警弹窗（debounce 限频：告警风暴时合并，避免堆叠一屏通知）
const notifyAlarm = debounce((payload: any) => {
  ElNotification({
    title: `告警：${payload.ruleName}`,
    message: payload.message,
    type: payload.level === 'critical' ? 'error' : 'warning',
    duration: 8000,
    onClick: () => router.push('/console/alarms')
  })
}, 800)
function onMsg(msg: any) {
  if (msg.type === 'alarm') notifyAlarm(msg.payload)
}

const tier = ref(localStorage.getItem('tier') || 'primary')
const perm = ref(localStorage.getItem('perm') || 'operate')

// ---- 平台公告 ----
const announcements = ref<Announcement[]>([])
const annVisible = ref(false)
// 已读最新公告 id 用响应式 ref 驱动红点（localStorage 非响应式，直接读会导致已读后红点不消失）
const readAnnId = ref(Number(localStorage.getItem('announcementReadId') || 0))
const unreadCount = computed(() => announcements.value.filter((a) => a.id > readAnnId.value).length)

// 公告内容为超管维护的 markdown（可信内容），marked 渲染
const md = (text: string) => marked.parse(text) as string
const formatTime = (t: string | null) => (t ? new Date(t).toLocaleString('zh-CN') : '')

async function loadAnnouncements() {
  try {
    announcements.value = (await api.announcements.list()) || []
  } catch { /* 公告加载失败不阻塞页面 */ }
}

function openAnnouncements() {
  annVisible.value = true
  // 标记当前所有公告为已读（红点立即消失）
  if (announcements.value.length) {
    readAnnId.value = announcements.value[0].id
    localStorage.setItem('announcementReadId', String(readAnnId.value))
    // 默认展开第一条，方便直接看最新公告
    expandedAnn.value = announcements.value[0].id
  }
}

// 公告详情展开/折叠（点标题行切换）
const expandedAnn = ref<number | null>(null)
function toggleAnn(id: number) {
  expandedAnn.value = expandedAnn.value === id ? null : id
}

onMounted(async () => {
  realtime.on(onMsg)
  loadAnnouncements()
  // 拉取最新 tier/权限：管理员调整二级权限后，二级刷新页面即生效
  try {
    const p = await api.profile()
    tier.value = p.tier || 'primary'
    perm.value = p.permission || 'operate'
    localStorage.setItem('tier', tier.value)
    localStorage.setItem('perm', perm.value)
  } catch { /* ignore */ }
})
onUnmounted(() => { realtime.off(onMsg) })

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
    localStorage.removeItem('userId')
    localStorage.removeItem('username')
    localStorage.removeItem('tier')
    localStorage.removeItem('perm')
    router.push('/login')
  }
}
</script>

<style scoped>
.layout { height: 100%; }
.aside {
  background: #001529; transition: width .2s ease;
  display: flex; flex-direction: column; overflow-x: hidden;
}
.logo {
  height: 60px; display: flex; align-items: center; justify-content: center; gap: 8px;
  color: #fff; font-size: 18px; font-weight: 600; white-space: nowrap; flex-shrink: 0;
}
.aside :deep(.el-menu) { border-right: none; flex: 1; overflow-y: auto; overflow-x: hidden; }
.aside :deep(.el-menu-item.is-active) {
  background: linear-gradient(90deg, #1668dc 0%, #12406e 100%);
  border-right: 3px solid #409EFF;
}
.aside :deep(.el-menu-item:hover),
.aside :deep(.el-sub-menu__title:hover) { background: #1f3a5f; }
.docs-link {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  margin: 8px 12px; padding: 0 20px; height: 40px;
  color: #a6adb4; font-size: 14px; text-decoration: none; white-space: nowrap;
  border-radius: 4px; transition: background .2s, color .2s; flex-shrink: 0;
}
.docs-link:hover { background: #1f3a5f; color: #fff; }
.docs-link .external { margin-left: auto; font-size: 12px; opacity: .6; }
.header {
  display: flex; align-items: center; justify-content: space-between;
  background: #fff; border-bottom: 1px solid #e8e8e8;
}
.header-left { display: flex; align-items: center; gap: 14px; }
.header-right { display: flex; align-items: center; gap: 18px; }
.bell { cursor: pointer; color: #606266; transition: color .2s; }
.bell:hover { color: #409EFF; }
.ann-badge { display: flex; align-items: center; }
.ann-empty { text-align: center; color: #999; padding: 24px 0; }
.ann-item { padding: 4px 0; border-bottom: 1px solid #f0f0f0; }
.ann-item:last-child { border-bottom: none; }
.ann-item.important .ann-title { color: #f56c6c; }
.ann-row {
  display: flex; align-items: center; gap: 8px; padding: 10px 8px;
  cursor: pointer; border-radius: 4px; transition: background 0.15s;
}
.ann-row:hover { background: #f5f7fa; }
.ann-title { font-weight: 600; color: #303133; }
.ann-meta { margin-left: auto; color: #c0c4cc; font-size: 12px; }
.ann-arrow { color: #c0c4cc; font-size: 12px; }
.ann-content {
  color: #606266; font-size: 13px; line-height: 1.7;
  padding: 8px 12px 12px; background: #fafafa; border-radius: 4px; margin: 0 8px 8px;
}
.ann-content :deep(p) { margin: 4px 0; }
.ann-content :deep(h1), .ann-content :deep(h2), .ann-content :deep(h3) { margin: 10px 0 4px; }
.ann-content :deep(ul), .ann-content :deep(ol) { padding-left: 20px; }
.ann-content :deep(code) { background: #f0f2f5; padding: 1px 4px; border-radius: 3px; }
.collapse-btn { cursor: pointer; color: #606266; transition: color .2s; }
.collapse-btn:hover { color: #409EFF; }
.user { display: flex; align-items: center; gap: 6px; cursor: pointer; color: #333; }
.avatar {
  width: 28px; height: 28px; border-radius: 50%;
  background: linear-gradient(135deg, #409EFF, #1668dc); color: #fff;
  font-size: 14px; font-weight: 600;
  display: flex; align-items: center; justify-content: center;
}
.main { background: #f0f2f5; }
</style>
