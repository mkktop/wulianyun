---
layout: home

hero:
  name: KK 物联云
  text: 设备接入开发文档
  tagline: 对标 OneNET / 天翼 / 有人云，三协议接入、物模型、规则引擎、OTA、开放平台
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/overview
    - theme: alt
      text: MQTT 接入
      link: /guide/mqtt

features:
  - icon: 📡
    title: 三协议接入
    details: MQTT（EMQX）/ TCP 透传 DTU / HTTP 直传，统一遥测管线，物模型校验。
    link: /guide/mqtt
    linkText: 查看接入方式
  - icon: 🧩
    title: 物模型 TSL
    details: properties / events / services 三类能力，类型与范围校验，JSON 导入导出。
    link: /guide/tsl
    linkText: 物模型协议
  - icon: ⬇️
    title: 下行控制与影子
    details: 透传命令 / 属性设置 / 服务调用，设备影子离线补发，指令应答状态机。
    link: /guide/downlink-shadow
    linkText: 下行协议
  - icon: 🚀
    title: OTA 升级
    details: 固件 SHA-256 校验、批量任务下发、设备进度回报。
    link: /guide/ota
    linkText: OTA 协议
  - icon: 🔑
    title: 开放平台 OpenAPI
    details: HMAC-SHA256 签名，设备查询 / 实时数据 / 命令下发，复用管理端能力。
    link: /guide/openapi
    linkText: OpenAPI 文档
  - icon: 🛡️
    title: 一机一密 / 一型一密
    details: "支持动态注册（免预注册建号）与 tk: 动态令牌，主题级 ACL 授权。"
    link: /guide/auth-security
    linkText: 认证与安全
---
