# 超管后台（系统管理）

> 平台超管（admin 角色）专属后台：系统状态、参数配置、公告管理、帮助中心、全量用户管理、全局日志检索。
> 入口：控制台侧边栏「系统管理」（仅 admin 角色可见，普通账号访问接口返回 403）。

## 一、登录与权限

- 与普通账号使用同一登录入口；`admin` 角色登录后侧边栏出现「系统管理」分组
- 所有 `/admin/*` 接口由 `AdminAuth` 中间件保护，仅 `role == "admin"` 可访问（前端菜单隐藏仅是体验，后端兜底拦截）
- 超管看所有账号的数据（数据隔离中的 `ownedScope` 对超管不过滤）

## 二、功能清单

### 1. 系统状态（`/console/system/status`）

| 指标 | 说明 |
|---|---|
| 服务版本 | 构建时 `-ldflags -X iot-platform/internal/api.Version=<VERSION>` 注入 |
| 运行时长 / Go 版本 / Goroutines / 内存 | 进程运行时信息 |
| 数据库 / Redis | 健康探测（异常时显示错误信息） |
| MQTT Broker | 平台内部客户端连接状态 |
| TCP 网关 | 当前在线连接数 |
| EMQX 规则引擎 | 是否启用快路径 |
| 全局统计 | 用户/产品/设备/在线/今日消息（admin 视角，不过滤账号） |

页面 30s 自动刷新。

### 2. 参数配置（`/console/system/config`）

**热更新参数**（存 `system_settings` 表，修改后立即生效，无需重启）：

| 参数 | 类型 | 说明 |
|---|---|---|
| `register_enabled` | bool | 开放注册开关，false 时注册接口拒绝新账号 |
| `jwt_expire_hours` | int | 登录令牌有效期（小时），新登录生效 |
| `trace_retention_days` | int | 消息轨迹保留天数（清理循环每次执行时读取） |
| `device_log_retention_days` | int | 设备日志保留天数 |

置空可恢复为 config.yaml 默认值。

**只读配置**：下方折叠面板展示当前生效的完整配置（jwt secret / 密码 / 存储密钥等敏感项已打码）。
基础设施参数（storage / mqtt / database / gateway / redis 等）依赖启动时初始化，**不支持热改**，
修改需编辑 `config.yaml` 并重启服务。

### 3. 公告管理（`/console/system/announcements`）

- 公告支持 markdown，草稿 / 发布两级状态，可标记「重要」
- 发布后所有登录账号可见：控制台顶部铃铛（新公告红点角标）+ 平台概览页公告区块
- 接口：`GET/POST /admin/announcements`、`PUT/DELETE /admin/announcements/:id`
- 用户侧读取：`GET /announcements`（仅已发布）

### 4. 帮助中心（`/console/system/help-docs`）

- 超管在线编辑 markdown 文档（`key` 为英文唯一标识），登录账号可通过 `GET /help-docs/:key` 读取
- 协议文档（docs/ VitePress 静态站）不在此管理，仍走构建发布流程

### 5. 用户管理（`/console/system/users`）

- 全量用户（超管 / 一级 / 二级），支持用户名/昵称搜索、角色/状态过滤、分页
- 可创建用户（普通一级或超管）、编辑昵称/角色/权限、启用/禁用、重置密码、删除
- 安全约束：不能删除/禁用/降级自己；删除前需名下无设备且无子账号（与一级管二级的约束一致）

### 6. 全局日志（`/console/system/logs`）

- **设备日志**：按设备（名称搜索选择）/ 分类 / 内容关键词（payload 模糊）检索，查看 payload 详情
- **消息轨迹**：按 TraceID / 设备 / 状态 / 时间范围检索，抽屉查看分阶段耗时与原始报文
- 超管视角可跨账号检索任意设备

## 三、接口一览

```
GET    /admin/system/status        系统状态（版本/健康/全局统计）
GET    /admin/system/config        当前生效配置（只读，敏感项打码）
GET    /admin/system/settings      热更新参数列表
PUT    /admin/system/settings      更新热更新参数
GET    /admin/announcements        公告列表（含草稿）
POST   /admin/announcements        新建公告
PUT    /admin/announcements/:id    修改/发布/下线
DELETE /admin/announcements/:id    删除公告
GET    /admin/help-docs            帮助文档列表
POST   /admin/help-docs            新建帮助文档
PUT    /admin/help-docs/:id        更新帮助文档
DELETE /admin/help-docs/:id        删除帮助文档
GET    /admin/users                全量用户分页列表
POST   /admin/users                创建用户（一级/超管）
PUT    /admin/users/:id            修改用户（昵称/角色/状态/权限/重置密码）
DELETE /admin/users/:id            删除用户
```
