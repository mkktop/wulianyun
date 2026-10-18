# KK 物联云 · 主流平台能力对标审查报告

- **审查日期**：2026-08-19
- **审查对象**：本仓库 @ master（VERSION `1.0.10`，tags 至 v1.0.9）
- **审查方法**：以**代码为准**（非 README 声称）——后端 `server/internal/*` 全量路由/服务盘点 + 前端 `web/src` 全量页面盘点 + 交叉验证；对标基准取**阿里云物联网平台、AWS IoT Core、Azure IoT Hub、ThingsBoard、EMQX** 的公开能力集。
- **图例**：✅ 达到主流水平 · 🟡 部分具备/有短板 · ❌ 缺失

---

## 一、总体结论

| 维度 | 结论 |
|---|---|
| 综合定位 | 单机可水平扩展的**私有化 IoT PaaS**，接入/物模型/影子/规则/告警/OTA 的**主干链路完整且工程质量高**，开发者工具与离线交付是同类开源少有的亮点 |
| 对标 ThingsBoard CE | 约七成：接入与设备管理持平甚至更顺（中文/DTU/Modbus 场景），**可视化（自定义 Dashboard）与多租户/RBAC 明显落后** |
| 对标 阿里云 IoT / AWS IoT Core | 约五成：核心设备链路齐全，缺 **CoAP/LwM2M、X.509/mTLS、服务端订阅推送、官方 SDK、聚合查询、通知渠道矩阵** |
| 最短补齐路径 | P0 六项（本报告第五节）均为低成本高收益，可在一个迭代内把"标配感"补齐 |

---

## 二、能力矩阵

| # | 能力域 | 评级 | 现状要点（证据） |
|---|---|---|---|
| 1 | 接入通道质量（MQTT/TCP-DTU/HTTP） | ✅ | 三路统一汇入 `service.HandleTelemetry`（ingest.go:179+）；TCP 组帧/心跳/限流、EMQX HTTP 回调认证+主题 ACL |
| 2 | 接入协议广度 | ❌ | 仅 MQTT/TCP/HTTP；无 CoAP、LwM2M、LoRaWAN（全库 grep 零命中）——NB-IoT/低功耗场景无法覆盖 |
| 3 | 设备认证与密钥体系 | 🟡 | 一机一密/一型一密动态注册/`tk:` 动态 Token 齐全；**无 X.509 证书/mTLS 设备认证**（TCP 网关 TLS 无 ClientAuth，EMQX 认证器为 password_based/http） |
| 4 | 物模型 TSL | ✅ | properties/events/services + JSON 导入导出 + 保留字校验；对标阿里云物模型基本形态 |
| 5 | 自定义协议解析（透传） | ✅ | goja `decode/encode`、200ms 超时、VM 池化、脚本哈希缓存、hex 在线测试 |
| 6 | 设备影子 | ✅ | desired/reported、delta 下发、上线补发（retained）、达成自动清除、版本号 |
| 7 | 指令应答闭环 | 🟡 | messageId pending→acked 幂等翻转（ingest.go:456-465）；**超时状态定义了但无扫描器**，pending 永挂 |
| 8 | OTA 升级 | 🟡 | SHA-256+CRC32 校验、批量任务、进度上报；**无灰度分批、无任务内设备级进度/重试/取消、无差分升级、离线设备不补送** |
| 9 | 远程配置/NTP/广播/网关子设备 | ✅ | config.get/push 版本化、ntp.request/response、产品级广播、子设备 login/logout 一型一密动态注册（单层拓扑） |
| 10 | Modbus 云端轮询 | ✅ | 分频采集组、批量合并读（maxGap=8/上限120）、onchange、分布式锁、半双工串行——**特色能力，多数开源平台无云端 Modbus 轮询** |
| 11 | 规则引擎 | 🟡 | 阈值/离线/转发三类 + 复合条件 + 静默期 + 自动恢复；转发仅 webhook/MQTT 桥接，**Kafka 占位未实现**（api/rule.go:28-40 直接拒绝）；无规则流/JS transform/执行日志 |
| 12 | 告警体系 | 🟡 | 阈值+离线+自动恢复+确认闭环+趋势统计完整；**通知渠道仅站内 WS + Webhook**（rule/engine.go:150-151），无邮件/短信/钉钉/企微/飞书 |
| 13 | 遥测存储 | ✅ | TimescaleDB 超表 + 自动保留/压缩 + 无扩展非致命降级；批量缓冲（100条/1s） |
| 14 | 数据查询与导出 | 🟡 | 原始点查询 + 设备级 CSV 导出（≤31天、防公式注入）；**无聚合/降采样查询**（全库无 time_bucket） |
| 15 | 设备时间戳 | ❌ | 遥测 ts 固定取服务器 `time.Now()`（ingest.go:207,261），**不支持设备自带 ts**——DTU 断网缓存补传会丢失原始时间 |
| 16 | 可视化 | ❌ | 固定概览页 + 固定大屏模板；**无自定义 Dashboard/widget 库/拖拽布局**（对标 ThingsBoard 最大差距）；ECharts 仅注册 Line/Bar/Pie，无地图/GIS/组态 |
| 17 | 开放平台 OpenAPI | 🟡 | HMAC-SHA256 签名规范（body 哈希+nonce 防重放，工程上优于不少同类）；**仅 7 端点**（5 读 2 写，router.go:231-240），无产品/规则/告警/OTA 开放，无服务端订阅推送，无官方 SDK |
| 18 | 多租户与 RBAC | 🟡 | 两级账号 + 产品下放 + operate/view 两档权限；**无组织/租户模型、无自定义角色、无设备级授权**（对标阿里云 RAM 差距大） |
| 19 | 安全基线 | ✅ | 常量时间比较、SSRF 含 DNS 解析后拦截+重定向逐跳复检、CSV 注入防护、密码版本即时撤销 JWT、nonce 防重放、限流与代理防伪造——**同类中扎实** |
| 20 | 可观测性 | 🟡 | healthz/readyz 双探针 + 超管状态页 + 消息轨迹分阶段耗时 + 设备日志；**无 Prometheus /metrics、无 pprof**（人读页面不可抓取） |
| 21 | 部署与交付 | ✅✅ | 离线交付包（install/upgrade/backup/restore/diag + sha256 + 固定镜像 tag）+ CI Release；私有化商业化场景**超越多数开源** |
| 22 | 边缘计算 | ❌ | 无边缘 agent/边缘规则（AWS Greengrass / Azure IoT Edge / ThingsBoard Edge 级能力） |
| 23 | 国际化 | ❌ | 前后端纯中文硬编码，无 vue-i18n、无错误码目录（国内交付可接受，出海不可） |

---

## 三、已达到或超越主流的能力（亮点，保持即可）

1. **开发者工具四件套**：内存态设备模拟器（api/simulator.go）、超管 MQTT 调试台（api/mqtt_debug.go）、消息轨迹 TraceID 分阶段耗时（model/trace.go 含 ingest/decode/store/rule 毫秒）、分类设备日志——阿里云有等价物，ThingsBoard CE 明显弱于此组合。
2. **DTU/透传/Modbus 场景深度**：注册包（三元组或 IMEI/ICCID 注册码）→ 组帧 → JS 编解码 → 云端 Modbus 轮询（分频+批量读+onchange），是面向中国工业 DTU 市场的完整闭环，开源对标物少。
3. **多实例一致性设计**：WS 扇出、Modbus 轮询锁、静默期共享、TCP 下行路由均走 Redis 且可降级单机——架构成熟度高于多数单机开源。
4. **安全细节**：OpenAPI nonce 防重放 + body 哈希入签、webhook SSRF 的 DNS 解析级防护、CSV 公式注入防护、密码版本即时撤销——审查中未发现同类常见的低级漏洞。
5. **私有化交付工程**：build.sh 固定镜像 tag、uid 10001 权限处理、升级不动数据卷、备份含 redis RDB、diag 脱敏——商业交付门槛已过。

---

## 四、与主流平台的差距清单

### A. 接入与认证

| 差距 | 现状（证据） | 主流参照 | 建议 |
|---|---|---|---|
| 无 CoAP | grep 零命中 | 阿里云/AWS IoT 均支持（NB-IoT/低功耗） | P2：EMQX 有 CoAP 网关插件，可复用回调鉴权 |
| 无 LwM2M | grep 零命中 | 阿里云（尊享）、AWS IoT（IoT Core for LwM2M 挂了但生态在） | P2：评估 EMQX LwM2M 网关 |
| 无 X.509/mTLS 设备认证 | TCP 网关 TLS 无 ClientAuth（gateway/tcp.go:278-285）；EMQX 用 password_based/http（deploy/emqx/emqx.conf:21-22） | AWS/Azure 主推 X.509；EMQX 原生 x509 认证器 | P2：EMQX 侧切 x509 认证器 + 平台补证书签发/吊销管理 |
| HTTP 直传不支持 `tk:` token | device_http.go 仅 Base64 三元组 | — | P1：复用 token_auth.go，小改动 |
| 遥测不支持设备自带时间戳 | ts 固定 `time.Now()`（ingest.go:207,261） | 阿里云 ALink `time` 字段、ThingsBoard device ts | **P0**：payload 带 `ts` 时优先采用（断网补传刚需） |

### B. 设备管理

| 差距 | 现状（证据） | 建议 |
|---|---|---|
| 标签不可过滤 | Device.Tags 字段有（model/model.go:121），ListDevices 仅 productId/keyword/status/groupId（api/device.go:103-116） | **P0**：list 加 tags jsonb 包含过滤 |
| 无批量禁用/删除/移动分组 | 仅批量创建（上限 500，api/ops.go:180-238） | P0 |
| 无生命周期流转 | 四状态 + 上下线事件，无激活/注销/退役 | P2（按客户诉求） |
| 子设备仅一层拓扑 | gateway_id 单字段 | P2 |

### C. 规则/告警/数据

| 差距 | 现状（证据） | 建议 |
|---|---|---|
| 无邮件/短信/钉钉/企微/飞书通知 | action.notify 仅 `ws`/`webhook`（rule/engine.go:150-151） | **P0**：钉钉/企微/飞书机器人均为 HTTP POST，复用 postJSONWithRetry；邮件加 SMTP |
| Kafka 转发占位 | API 拒绝创建（api/rule.go:28-40），README 却声称支持 | P0（文档已修）：要么实现要么从 UI 选项中隐藏 |
| 无聚合/降采样查询 | DeviceHistory 无 interval/agg 参数（api/data.go:29-74），全库无 time_bucket | **P0**：TimescaleDB `time_bucket` 一行 SQL + 前端曲线自动降采样 |
| 无规则 transform/规则流/执行日志 | 条件仅比较运算 | P1/P2 |
| 告警无处理人/工单流 | 仅 confirm/resolve | P2 |

### D. 可视化与开放

| 差距 | 现状（证据） | 建议 |
|---|---|---|
| 无自定义 Dashboard | 概览/大屏均为写死模板（BigScreen.vue 187 行全硬编码）；全 src 无 grid-layout/拖拽 | **P1**：widget JSON + 网格布局 v1（对标 ThingsBoard 的核心卖点，是最大 UI 差距） |
| 无地图/GIS/组态 | ECharts 仅注册 Line/Bar/Pie（utils/echarts.ts:6-11） | P2（按行业客户） |
| OpenAPI 仅 7 端点 | 5 读 2 写，无产品/规则/告警/OTA（router.go:231-240） | **P1**：复用管理端 handler 低成本全量化 |
| 无服务端订阅推送 | 第三方应用只能轮询 OpenAPI；规则 forward 面向数据集成不面向应用 | **P1**：设备遥测/上下线事件的应用级 webhook 订阅（注册回调 URL + 重试） |
| 无官方 SDK | docs 仅报文格式 + 签名说明 | P2：先出 Go/Node 签名客户端 + 示例 |

### E. 权限与租户

| 差距 | 现状（证据） | 建议 |
|---|---|---|
| 无自定义角色 | role 固定 admin/user；二级账号仅 operate/view 两档（model/model.go:23-42） | P1：角色=权限点集合，路由中间件按权限点校验 |
| 无组织/多租户 | "租户"= User 两层 ParentID | P2（涉及数据模型改造，先按项目/客户分层） |
| 授权粒度=产品级 | ProductGrant（model/model.go:45-52），无设备级 | P2 |

### F. 运维与工程

| 差距 | 现状（证据） | 建议 |
|---|---|---|
| 无 Prometheus /metrics、pprof | grep 零命中，go.mod 无 prometheus | **P1**：标准库 + prometheus client 半天量级 |
| 指令 pending 无超时扫描 | status 定义 timeout 但无写入（grep 零命中） | **P0**：复用离线巡检定时器模式，pending > N 分钟置 timeout |
| OTA 无灰度/钻取/重试取消 | OTA.vue 仅聚合进度（OTA.vue:70-74） | P1 |
| 无版本化 migration | 裸 AutoMigrate（repository/db.go:58-72） | P2（商业二期，CLAUDE.md 已列） |
| 无 license 授权 | grep 零命中 | P2（商业二期） |
| 无 i18n | 前后端中文硬编码 | P2（出海才需要） |
| 无边缘计算 | — | P2：DTU 场景优先做"断网续传"约定（配合设备 ts）而非完整边缘运行时 |

---

## 五、优先级路线图（对标主流的最短路径）

**P0（低成本补"标配感"，单迭代内可完成）**
1. 遥测聚合查询（time_bucket + agg 参数 + 前端联动）
2. 设备自带 `ts` 支持（断网补传）
3. 告警通知渠道：SMTP 邮件 + 钉钉/企微/飞书机器人（复用 webhook 基建）
4. 设备标签过滤 + 批量禁用/删除/移动分组
5. 指令超时扫描器
6. 文档与实现对齐（本次审查已修正 4 处，见下节）

**P1（主流核心能力，每项约 3~10 天）**
7. OpenAPI 全量化 + 应用级事件订阅推送（webhook 注册）
8. Prometheus /metrics + pprof
9. 自定义仪表盘 v1（widget + 布局 JSON + ECharts 组件集）
10. OTA 任务增强（设备级进度/取消/重试/分批灰度）
11. 自定义角色 RBAC
12. HTTP 直传支持 `tk:` token；帮助中心前台渲染页（打通"可写不可读"）

**P2（差异化/长周期）**
13. CoAP（EMQX 网关复用）→ LwM2M 评估
14. X.509/mTLS 设备认证 + 证书管理
15. 规则引擎 transform 节点与执行日志
16. 多租户/组织模型；设备级授权
17. 边缘：DTU 断网续传协议（配合 #2）
18. 商业二期（CLAUDE.md 已列）：版本化 migration、license、arm64 实测、捆绑 Docker 离线安装
19. i18n（出海交付时）

---

## 六、审查中发现的文档与实现不一致（已修正）

| # | 位置 | 问题 | 处置 |
|---|---|---|---|
| 1 | README.md:87,369 | 声称 Kafka 转发目标，代码 API 层直接拒绝（api/rule.go:28-40） | 已改注"暂未实现" |
| 2 | README.md:399 | "签名仅覆盖 appKey+timestamp、不含请求体"与上文签名公式矛盾且滞后（实际含 body 哈希 + nonce 防重放，openapi.go:106-209） | 已更正 |
| 3 | README.md:581 | SSRF 描述停留在"仅字面量校验"；代码已在 DNS 解析后拦截 + 重定向逐跳复检（engine.go:626-659） | 已更正（代码比文档强） |
| 4 | CLAUDE.md:124,139 | VERSION 写 1.0.4（实际 1.0.10）；"ldflags 版本注入未做"已过时（build.sh:74 已注入） | 已更正 |

**其余滞后观察（未改，待决策）**：帮助中心库内文档"可写不可读"（公开读取 API 有、前端无渲染页）；Pinia 已安装未使用；物模型编辑器事件 outputs/服务 inputs/outputs 表单未实现（ThingModelEditor.vue:163-168 只给空数组）；测试机实际运行版本请以服务器为准。

---

*本报告由代码级全量盘点生成（后端路由表、前端页面清单均有 file:line 证据），可作为下一迭代的规划输入。*
