import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import { pending, reqKey } from '../utils/http-pending'

const http = axios.create({ baseURL: '/api/v1', timeout: 15000 })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  // GET 去重：相同请求 later-wins（取消前一个），
  // 并挂 AbortController.signal，供路由切换时统一取消，消除快速切页竞态。
  const method = (config.method || 'get').toLowerCase()
  if (method === 'get') {
    const key = reqKey(method, config.url, config.params)
    pending.get(key)?.abort()
    const ctrl = new AbortController()
    pending.set(key, ctrl)
    config.signal = ctrl.signal
  }
  return config
})

http.interceptors.response.use(
  (resp) => {
    if (resp.config) pending.delete(reqKey(resp.config.method || 'get', resp.config.url, resp.config.params))
    // Blob 响应（文件下载）直接返回，不做 JSON 解包
    if (resp.data instanceof Blob) return resp.data
    const { code, msg, data } = resp.data
    if (code !== 0) {
      ElMessage.error(msg || '请求失败')
      return Promise.reject(new Error(msg))
    }
    return data
  },
  (err) => {
    if (err.config) pending.delete(reqKey(err.config.method || 'get', err.config.url, err.config.params))
    // 主动取消（去重 / 路由切换）静默处理，不弹错误提示
    if (axios.isCancel(err) || err.code === 'ERR_CANCELED') {
      return Promise.reject(err)
    }
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
    } else {
      ElMessage.error(err.message || '网络错误')
    }
    return Promise.reject(err)
  }
)

// ---- 类型 ----
export interface Product {
  id: number
  name: string
  productKey: string
  protocol: string
  dataFormat: string
  accessMode: string
  secretMode: string
  productSecret: string
  pollInterval: number
  description: string
  deviceCount: number
  createdAt: string
  // TCP 组帧/心跳
  frameMode: string
  frameDelimiter: string
  frameLenOffset: number
  frameLenSize: number
  frameLenAdjust: number
  heartbeatPacket: string
  heartbeatReply: string
  configVersion: number
}

export interface TslEnumItem {
  value: number
  label: string
}

export interface TslParam {
  identifier: string
  name: string
  dataType: string
}

export interface TslEvent {
  identifier: string
  name: string
  type: string
  outputs: TslParam[]
  desc: string
}

export interface ModbusPoint {
  id?: number
  identifier: string
  name: string
  groupId: number
  slaveId: number
  functionCode: number
  address: number
  rawType: string
  bitPosition: number
  scale: number
  offset: number
  swapByte: boolean
  swapWord: boolean
  accessMode: string
  unit: string
}

export interface ModbusGroup {
  id: number
  productId: number
  name: string
  pollInterval: number
  reportMode: string
  pointCount: number
}

export interface Device {
  id: number
  productId: number
  productKey: string
  productName: string
  name: string
  secret: string
  regCode: string
  status: string
  groupId: number
  groupName: string
  tags: string[] | string
  remark: string
  isGateway: boolean
  gatewayId?: number
  lastOnlineAt: string | null
  lastOfflineAt: string | null
  createdAt: string
}

export interface Page<T> {
  total: number
  list: T[]
}

export interface TslProperty {
  identifier: string
  name: string
  dataType: string
  unit: string
  min: number | null
  max: number | null
  step: number | null
  accessMode: string
  enumSpec: TslEnumItem[]
  desc: string
}

export interface TslService {
  identifier: string
  name: string
  async: boolean
  inputs: TslParam[]
  outputs: TslParam[]
  desc: string
}

export interface Rule {
  id: number
  name: string
  type: string
  productId: number
  deviceId: number
  productName: string
  deviceName: string
  condition: any
  action: any
  silence: number
  enabled: boolean
  createdAt: string
}

export interface Alarm {
  id: number
  ruleName: string
  deviceId: number
  deviceName: string
  level: string
  message: string
  status: string
  createdAt: string
  resolvedAt: string | null
  confirmedAt: string | null
}

export interface OpenApp {
  id: number
  name: string
  appKey: string
  appSecret: string
  enabled: boolean
  createdAt: string
}

export interface Account {
  id: number
  username: string
  nickname: string
  role: string
  parentId?: number | null
  status: string
  permission: string
  createdAt: string
  deviceCount?: number
  grantCount?: number
}

export interface ProductGrant {
  id: number
  productId: number
  secondaryId: number
  secondaryName?: string
  nickname?: string
  permission: string
  createdAt: string
}

// 账号层级：platform(超管) / primary(一级) / secondary(二级)
export function computeTier(user: any): string {
  if (user?.role === 'admin') return 'platform'
  if (user?.parentId == null) return 'primary'
  return 'secondary'
}
export function currentTier(): string {
  return localStorage.getItem('tier') || 'primary'
}
export function isSecondary(): boolean {
  return currentTier() === 'secondary'
}
// 只读账号（P2 账号内 RBAC）：无写操作权限，前端据此隐藏写按钮，后端 RequireOperate 兜底
export function isViewOnly(): boolean {
  return localStorage.getItem('perm') === 'view'
}

export interface EventReport {
  id: number
  deviceId: number
  deviceName: string
  identifier: string
  type: string
  params: any
  createdAt: string
}

export interface CommandLog {
  id: number
  deviceId: number
  deviceName: string
  channel: string
  payload: string
  success: boolean
  error: string
  createdAt: string
}

export interface DeviceGroup {
  id: number
  name: string
  description: string
  deviceCount: number
  createdAt: string
}

// ---- 接口 ----
export const api = {
  login: (data: { username: string; password: string }) =>
    http.post('/auth/login', data) as Promise<{ token: string; user: any }>,
  register: (data: { username: string; password: string; nickname?: string }) =>
    http.post('/auth/register', data) as Promise<{ id: number }>,
  changePassword: (data: { oldPassword: string; newPassword: string }) =>
    http.post('/auth/change-password', data) as Promise<void>,
  profile: () => http.get('/auth/profile') as Promise<any>,

  listAccounts: (params?: any) => http.get('/accounts', { params }) as Promise<any>,
  createAccount: (data: { username: string; password: string; nickname?: string }) =>
    http.post('/accounts', data) as Promise<Account>,
  updateAccount: (id: number, data: { nickname?: string; password?: string; status?: string }) =>
    http.put(`/accounts/${id}`, data),
  deleteAccount: (id: number) => http.delete(`/accounts/${id}`),

  listGrants: (productId: number) => http.get(`/products/${productId}/grants`) as Promise<ProductGrant[]>,
  createGrant: (productId: number, secondaryId: number) =>
    http.post(`/products/${productId}/grants`, { secondaryId }),
  deleteGrant: (productId: number, secondaryId: number) =>
    http.delete(`/products/${productId}/grants/${secondaryId}`),
  overview: () => http.get('/overview') as Promise<any>,

  listProducts: (params?: any) => http.get('/products', { params }) as Promise<Page<Product>>,
  getProduct: (id: number | string) => http.get(`/products/${id}`) as Promise<Product>,
  createProduct: (data: any) => http.post('/products', data) as Promise<Product>,
  updateProduct: (id: number, data: any) => http.put(`/products/${id}`, data),
  deleteProduct: (id: number) => http.delete(`/products/${id}`),

  listDevices: (params?: any) => http.get('/devices', { params }) as Promise<Page<Device>>,
  getDevice: (id: number | string) => http.get(`/devices/${id}`) as Promise<Device>,
  createDevice: (data: any) => http.post('/devices', data) as Promise<Device>,
  updateDevice: (id: number, data: any) => http.put(`/devices/${id}`, data),
  deleteDevice: (id: number) => http.delete(`/devices/${id}`),
  deviceEvents: (id: number | string, params?: any) =>
    http.get(`/devices/${id}/events`, { params }) as Promise<Page<any>>,
  deviceLatest: (id: number | string) => http.get(`/devices/${id}/latest`) as Promise<any>,
  deviceHistory: (id: number | string, params?: any) =>
    http.get(`/devices/${id}/history`, { params }) as Promise<{ ts: number; data: Record<string, any> }[]>,
  sendCommand: (id: number | string, payload: any) => http.post(`/devices/${id}/command`, payload),

  getThingModel: (productId: number | string) =>
    http.get(`/products/${productId}/thing-model`) as Promise<{ properties: TslProperty[]; events: TslEvent[]; services: TslService[] }>,
  saveThingModel: (productId: number | string, data: { properties: TslProperty[]; events: TslEvent[]; services: TslService[] }) =>
    http.put(`/products/${productId}/thing-model`, data),

  getModbusPoints: (productId: number | string) =>
    http.get(`/products/${productId}/modbus-points`) as Promise<ModbusPoint[]>,
  saveModbusPoints: (productId: number | string, points: ModbusPoint[]) =>
    http.put(`/products/${productId}/modbus-points`, { points }),
  testModbusPoint: (productId: number | string, point: ModbusPoint, hexStr: string) =>
    http.post(`/products/${productId}/modbus-points/test`, { point, hex: hexStr }) as Promise<{ value: number }>,

  listModbusGroups: (productId: number | string) =>
    http.get(`/products/${productId}/modbus-groups`) as Promise<ModbusGroup[]>,
  createModbusGroup: (productId: number | string, data: any) =>
    http.post(`/products/${productId}/modbus-groups`, data) as Promise<ModbusGroup>,
  updateModbusGroup: (productId: number | string, gid: number, data: any) =>
    http.put(`/products/${productId}/modbus-groups/${gid}`, data),
  deleteModbusGroup: (productId: number | string, gid: number) =>
    http.delete(`/products/${productId}/modbus-groups/${gid}`),

  getShadow: (id: number | string) => http.get(`/devices/${id}/shadow`) as Promise<any>,
  setProperty: (id: number | string, params: any) => http.post(`/devices/${id}/property`, params) as Promise<any>,
  invokeService: (id: number | string, service: string, params: any) =>
    http.post(`/devices/${id}/service`, { service, params }),

  listRules: (params?: any) => http.get('/rules', { params }) as Promise<Page<Rule>>,
  createRule: (data: any) => http.post('/rules', data) as Promise<Rule>,
  updateRule: (id: number, data: any) => http.put(`/rules/${id}`, data),
  deleteRule: (id: number) => http.delete(`/rules/${id}`),

  listAlarms: (params?: any) => http.get('/alarms', { params }) as Promise<Page<Alarm>>,
  resolveAlarm: (id: number) => http.post(`/alarms/${id}/resolve`),
  confirmAlarm: (id: number) => http.post(`/alarms/${id}/confirm`),
  alarmStats: () => http.get('/alarms/stats') as Promise<{ total: number; firing: number; resolved: number; today: number }>,
  alarmTrend: () => http.get('/alarms/trend') as Promise<{ day: string; count: number }[]>,

  getCodec: (productId: number | string) => http.get(`/products/${productId}/codec`) as Promise<{ script: string }>,
  saveCodec: (productId: number | string, script: string) => http.put(`/products/${productId}/codec`, { script }),
  testCodec: (productId: number | string, script: string, hexStr: string) =>
    http.post(`/products/${productId}/codec/test`, { script, hex: hexStr }) as Promise<any>,

  listApps: (params?: any) => http.get('/apps', { params }) as Promise<Page<OpenApp>>,
  createApp: (name: string) => http.post('/apps', { name }) as Promise<OpenApp>,
  updateApp: (id: number, data: any) => http.put(`/apps/${id}`, data),
  deleteApp: (id: number) => http.delete(`/apps/${id}`),

  productStats: (id: number | string) => http.get(`/products/${id}/stats`) as Promise<any>,
  getRemoteConfig: (id: number | string) =>
    http.get(`/products/${id}/config`) as Promise<{ version: number; config: Record<string, any> }>,
  saveRemoteConfig: (id: number | string, config: Record<string, any>) =>
    http.put(`/products/${id}/config`, { config }) as Promise<{ version: number }>,
  pushRemoteConfig: (id: number | string) => http.post(`/products/${id}/config/push`),
  broadcastProduct: (id: number | string, payload: Record<string, any>) =>
    http.post(`/products/${id}/broadcast`, { payload }),
  batchCreateDevices: (productId: number | string, names: string[]) =>
    http.post(`/products/${productId}/devices/batch`, { names }) as Promise<{ created: number; failed: { name: string; reason: string }[] }>,

  listEventReports: (params?: any) => http.get('/event-reports', { params }) as Promise<Page<EventReport>>,
  listCommandLogs: (params?: any) => http.get('/command-logs', { params }) as Promise<Page<CommandLog>>,

  listGroups: () => http.get('/groups') as Promise<DeviceGroup[]>,
  createGroup: (data: any) => http.post('/groups', data) as Promise<DeviceGroup>,
  updateGroup: (id: number, data: any) => http.put(`/groups/${id}`, data),
  deleteGroup: (id: number) => http.delete(`/groups/${id}`),

  // 设备模拟器
  simulator: {
    connect: (data: { productId: number; deviceId: number }) => http.post('/simulator/connect', data),
    publish: (data: { sessionId: string; payload: any }) => http.post('/simulator/publish', data),
    disconnect: (sessionId: string) => http.post('/simulator/disconnect', { sessionId }),
    sessions: () => http.get('/simulator/sessions'),
  },
  // 消息轨迹
  traces: {
    list: (params: any) => http.get('/traces', { params }),
    get: (traceId: string) => http.get(`/traces/${traceId}`),
  },
  // 设备日志
  deviceLogs: {
    list: (deviceId: number, params: any) => http.get(`/devices/${deviceId}/logs`, { params }),
    listAll: (params: any) => http.get('/device-logs', { params }),
  },
  // 导出历史数据 CSV（按时间范围 + 参数过滤）
  exportHistory: (id: number | string, params: any) =>
    http.get(`/devices/${id}/export`, { params, responseType: 'blob' }) as Promise<Blob>,
  // MQTT 调试台（WebSocket 直连，此处仅保留占位）
  mqttDebug: {
    wsUrl: '/api/v1/mqtt-debug/ws',
  },
  // 物模型导入/导出
  tsl: {
    export: (productId: number) => http.get(`/products/${productId}/tsl/export`, { responseType: 'blob' }),
    import: (productId: number, file: File) => {
      const fd = new FormData()
      fd.append('file', file)
      return http.post(`/products/${productId}/tsl/import`, fd)
    },
  },
  // 统计
  stats: {
    overview: () => http.get('/stats/overview'),
    messageTrend: (params: any) => http.get('/stats/message-trend', { params }),
  },
  // OTA 固件升级
  ota: {
    firmwares: (params?: any) => http.get('/firmwares', { params }),
    uploadFirmware: (data: FormData) => http.post('/firmwares', data),
    deleteFirmware: (id: number) => http.delete(`/firmwares/${id}`),
    createTask: (data: any) => http.post('/ota-tasks', data),
    tasks: () => http.get('/ota-tasks'),
  },
  // 网关子设备
  gateway: {
    subDevices: (gatewayId: number) => http.get(`/devices/${gatewayId}/sub-devices`),
    addSubDevice: (gatewayId: number, data: any) => http.post(`/devices/${gatewayId}/sub-devices`, data),
    removeSubDevice: (gatewayId: number, deviceId: number) => http.delete(`/devices/${gatewayId}/sub-devices/${deviceId}`),
  },
}
