## 目标
为 KK物联云 新增一个参考 iot.10086.cn 风格的浅色科技蓝公开营销首页（Landing Page），放在根路径 `/`，控制台整体迁移到 `/console` 前缀，旧链接自动兼容。

## 路由调整（web/src/router/index.ts）
- 新增公开路由：`{ path: '/', component: Home }`（无需登录），并加入守卫白名单 `['/login', '/']`
- 控制台 Layout 组从 `/` 移到 `/console`（children 路径不变，解析为 /console/overview、/console/products …）
- 新增兜底兼容重定向：`/:pathMatch(.*)*` → `/console/<原路径>`，保证所有旧控制台链接（/overview、/devices、/products/:id 等，全项目 20+ 处 router.push）无需逐个修改即可继续工作
- 守卫：未登录访问 /console/* → /login；已登录访问 /login → /console/overview

## 配套小改
- **web/src/layout/Layout.vue**：侧边栏菜单 index 全部加 `/console` 前缀；`active` computed 剥离 `/console` 前缀后匹配（保持菜单高亮正确）
- **web/src/views/Login.vue**：登录成功跳转 `/` → `/console/overview`（否则登录后落在首页）
- **web/src/views/BigScreen.vue**：「退出大屏」按钮跳 `/overview` → `/console/overview`（一行）
- **web/index.html**：顺带补 meta description（可选，1 行）

## 新增页面 web/src/views/Home.vue（单文件，scoped 样式，无图片资源、纯 CSS/SVG 绘图）
浅色科技蓝：白底 + 品牌蓝 `#1677ff`/`#409EFF` + 青色渐变点缀，参考站式布局：

1. **顶栏**：内联 SVG 品牌 logo +「KK物联云」｜锚点导航（核心能力 / 接入方式 / 应用场景 / 开发资源）+「开发文档」(→ /developer/) +「进入控制台」(→ /console/overview) 按钮
2. **Hero 主视觉**：蓝青渐变背景 + 网格光效，主标语「一站式物联网云平台」，副标语「设备接入 · 数据管理 · 实时监控」，CTA 双按钮；下方用纯 CSS/SVG 画一个抽象设备拓扑图（中心云节点 + 设备节点 + 连线动画）
3. **能力数据条**：4 项静态能力数字（4 大接入协议 / 3 种数据模式 / 99.9% 高可用 / 7×24 运行）——因后端无公开免鉴权接口，不接真实数据，只做能力展示
4. **核心能力**：6 张卡片（多协议接入 MQTT/TCP/HTTP/Modbus · 物模型管理 · 规则引擎 · 告警中心 · OTA 升级 · 开放 API），文案取自项目 README 实际能力
5. **接入流程**：4 步引导（创建产品 → 定义物模型 → 接入设备 → 数据应用）
6. **应用场景**：3 个行业卡片（智能家居 / 工业设备监控 / 智慧农业·环境监测）
7. **开发者资源**：开发文档 / OpenAPI / 设备模拟器 / MQTT 调试台 入口卡片
8. **页脚**：版权信息 + 控制台/文档链接

响应式：CSS Grid/Flex + media query 适配移动端；锚点平滑滚动。

## 不做的事
- 不改后端（不加公开统计接口）；统计数据静态展示
- 不动 /developer/ 文档站、nginx 配置（SPA fallback 已覆盖 `/`）

## 验证
- `cd web && npm run build` 构建通过
- `npm run dev` 手动验证：未登录访问 `/` 见首页；点「进入控制台」跳登录；登录后进控制台；旧链接 /overview 重定向正常；菜单高亮正常