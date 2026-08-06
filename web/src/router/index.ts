import { createRouter, createWebHistory } from 'vue-router'
import { cancelAllPending } from '../utils/http-pending'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/Login.vue') },
    { path: '/screen', component: () => import('../views/BigScreen.vue') },
    {
      path: '/',
      component: () => import('../layout/Layout.vue'),
      redirect: '/overview',
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
        { path: 'tools/traces', component: () => import('../views/MessageTraces.vue'), meta: { title: '消息轨迹' } }
      ]
    }
  ]
})

router.beforeEach((to) => {
  // 路由切换：取消所有进行中的请求，避免旧页面请求结果覆盖新页面数据
  cancelAllPending()
  const token = localStorage.getItem('token')
  if (!token && to.path !== '/login') return '/login'
  if (token && to.path === '/login') return '/'
})

export default router
