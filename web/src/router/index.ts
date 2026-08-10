import { createRouter, createWebHistory } from 'vue-router'
import { cancelAllPending } from '../utils/http-pending'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue') },
    { path: '/screen', component: () => import('../views/BigScreen.vue') },
    // 公开官网首页（免登录）
    { path: '/', component: () => import('../views/Home.vue') },
    {
      path: '/console',
      component: () => import('../layout/Layout.vue'),
      redirect: '/console/overview',
      children: [
        { path: 'overview', component: () => import('../views/Overview.vue'), meta: { title: '平台概览' } },
        { path: 'products', component: () => import('../views/Products.vue'), meta: { title: '产品管理' } },
        { path: 'products/new', component: () => import('../views/ProductForm.vue'), meta: { title: '创建产品' } },
        { path: 'products/:id', component: () => import('../views/ProductDetail.vue'), meta: { title: '产品详情' } },
        { path: 'products/:id/edit', component: () => import('../views/ProductForm.vue'), meta: { title: '编辑产品' } },
        { path: 'devices', component: () => import('../views/Devices.vue'), meta: { title: '设备管理' } },
        { path: 'devices/:id', component: () => import('../views/DeviceDetail.vue'), meta: { title: '设备详情' } },
        { path: 'rules', component: () => import('../views/Rules.vue'), meta: { title: '规则引擎' } },
        { path: 'alarms', component: () => import('../views/Alarms.vue'), meta: { title: '告警中心' } },
        { path: 'apps', component: () => import('../views/Apps.vue'), meta: { title: '应用管理' } },
        { path: 'accounts', component: () => import('../views/Accounts.vue'), meta: { title: '子账号管理' } },
        { path: 'ota', component: () => import('../views/OTA.vue'), meta: { title: 'OTA升级' } },
        { path: 'tools/simulator', component: () => import('../views/DeviceSimulator.vue'), meta: { title: '设备模拟器' } },
        { path: 'tools/mqtt-debug', component: () => import('../views/MqttDebug.vue'), meta: { title: 'MQTT调试台' } },
        { path: 'tools/traces', component: () => import('../views/MessageTraces.vue'), meta: { title: '消息轨迹' } },
        // 平台超管专属后台（系统管理，仅 admin 角色可访问；后端 AdminAuth 兜底）
        { path: 'system/status', component: () => import('../views/system/SystemStatus.vue'), meta: { title: '系统状态', admin: true } },
        { path: 'system/config', component: () => import('../views/system/SystemConfig.vue'), meta: { title: '参数配置', admin: true } },
        { path: 'system/announcements', component: () => import('../views/system/Announcements.vue'), meta: { title: '公告管理', admin: true } },
        { path: 'system/help-docs', component: () => import('../views/system/HelpDocs.vue'), meta: { title: '帮助中心', admin: true } },
        { path: 'system/users', component: () => import('../views/system/AdminUsers.vue'), meta: { title: '用户管理', admin: true } },
        { path: 'system/logs', component: () => import('../views/system/SystemLogs.vue'), meta: { title: '全局日志', admin: true } },
        // /console 下未匹配路径（含旧链接转换来的）统一回落控制台首页，避免与顶层兜底构成重定向环
        { path: ':pathMatch(.*)*', redirect: '/console/overview' }
      ]
    },
    // 旧控制台链接兼容：/overview → /console/overview，/products/:id → /console/products/:id
    { path: '/:pathMatch(.*)*', redirect: (to) => `/console/${(to.params.pathMatch as string[]).join('/')}` }
  ]
})

router.beforeEach((to) => {
  // 路由切换：取消所有进行中的请求，避免旧页面请求结果覆盖新页面数据
  cancelAllPending()
  const token = localStorage.getItem('token')
  // 公开页面：/（官网首页）、/login；其余（含 /console/*）需要登录
  if (!token && to.path !== '/login' && to.path !== '/') return '/login'
  if (token && to.path === '/login') return '/console/overview'
  // 系统管理（admin 专属）页面：非超管账号重定向回控制台首页（后端 AdminAuth 兜底拦截）
  if (to.meta.admin && localStorage.getItem('tier') !== 'platform') {
    return '/console/overview'
  }
})

export default router
