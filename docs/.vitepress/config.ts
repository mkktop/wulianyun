import { defineConfig } from 'vitepress'

// 部署在 /developer/ 子路径；本地 dev 同样在该路径下访问
export default defineConfig({
  lang: 'zh-CN',
  title: 'KK 物联云',
  description: 'KK IoT Cloud · 设备接入协议与开放平台文档',
  base: '/developer/',
  lastUpdated: true,
  cleanUrls: true,

  head: [
    ['meta', { name: 'theme-color', content: '#409eff' }]
  ],

  themeConfig: {
    siteTitle: 'KK 物联云 · 开发文档',

    // 顶部导航
    nav: [
      { text: '快速开始', link: '/guide/overview' },
      { text: '接入协议', link: '/guide/mqtt' },
      { text: '开放平台', link: '/guide/openapi' },
      { text: '示例代码', link: '/guide/examples' }
    ],

    // 侧边栏：分组导航
    sidebar: {
      '/guide/': [
        {
          text: '开始',
          collapsed: false,
          items: [
            { text: '协议总览', link: '/guide/overview' },
            { text: '示例代码', link: '/guide/examples' }
          ]
        },
        {
          text: '设备接入协议',
          collapsed: false,
          items: [
            { text: 'MQTT 接入', link: '/guide/mqtt' },
            { text: 'TCP / DTU 接入', link: '/guide/tcp-dtu' },
            { text: 'HTTP 直传', link: '/guide/http' }
          ]
        },
        {
          text: '物模型与控制',
          collapsed: false,
          items: [
            { text: '物模型 TSL', link: '/guide/tsl' },
            { text: '下行控制与设备影子', link: '/guide/downlink-shadow' },
            { text: '网关子设备', link: '/guide/gateway' }
          ]
        },
        {
          text: '进阶',
          collapsed: false,
          items: [
            { text: 'OTA 升级', link: '/guide/ota' },
            { text: '开放平台 OpenAPI', link: '/guide/openapi' },
            { text: '认证与安全', link: '/guide/auth-security' }
          ]
        }
      ]
    },

    // 内置本地全文搜索
    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: {
            noResultsText: '无法找到相关结果',
            resetButtonTitle: '清除查询条件',
            footer: { selectText: '选择', navigateText: '切换', closeText: '关闭' }
          }
        }
      }
    },

    outline: {
      label: '本页导航',
      level: [2, 3]
    },

    docFooter: {
      prev: '上一篇',
      next: '下一篇'
    },

    lastUpdated: {
      text: '最后更新'
    },

    returnToTopLabel: '回到顶部',
    sidebarMenuLabel: '目录',
    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',

    footer: {
      message: '内容持续更新，如有疑问请联系平台技术支持',
      copyright: 'KK 物联云 · 设备接入文档'
    }
  }
})
