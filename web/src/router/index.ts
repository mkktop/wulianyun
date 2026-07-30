import { createRouter, createWebHistory } from 'vue-router'

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
        { path: 'apps', component: () => import('../views/Apps.vue'), meta: { title: '应用管理' } }
      ]
    }
  ]
})

router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  if (!token && to.path !== '/login') return '/login'
  if (token && to.path === '/login') return '/'
})

export default router
