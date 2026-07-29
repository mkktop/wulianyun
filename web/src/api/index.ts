import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'

const http = axios.create({ baseURL: '/api/v1', timeout: 15000 })

http.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

http.interceptors.response.use(
  (resp) => {
    const { code, msg, data } = resp.data
    if (code !== 0) {
      ElMessage.error(msg || '请求失败')
      return Promise.reject(new Error(msg))
    }
    return data
  },
  (err) => {
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

export interface Device {
  id: number
  productId: number
  productKey: string
  productName: string
  name: string
  secret: string
  status: string
  remark: string
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
}

export interface OpenApp {
  id: number
  name: string
  appKey: string
  appSecret: string
  enabled: boolean
  createdAt: string
}

// ---- 接口 ----
export const api = {
  login: (data: { username: string; password: string }) =>
    http.post('/auth/login', data) as Promise<{ token: string; user: any }>,
  register: (data: { username: string; password: string; nickname?: string }) =>
    http.post('/auth/register', data) as Promise<{ id: number }>,
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
  deviceEvents: (id: number | string) => http.get(`/devices/${id}/events`) as Promise<any[]>,
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
  alarmStats: () => http.get('/alarms/stats') as Promise<{ firing: number; today: number }>,

  getCodec: (productId: number | string) => http.get(`/products/${productId}/codec`) as Promise<{ script: string }>,
  saveCodec: (productId: number | string, script: string) => http.put(`/products/${productId}/codec`, { script }),
  testCodec: (productId: number | string, script: string, hexStr: string) =>
    http.post(`/products/${productId}/codec/test`, { script, hex: hexStr }) as Promise<any>,

  listApps: () => http.get('/apps') as Promise<OpenApp[]>,
  createApp: (name: string) => http.post('/apps', { name }) as Promise<OpenApp>,
  updateApp: (id: number, data: any) => http.put(`/apps/${id}`, data),
  deleteApp: (id: number) => http.delete(`/apps/${id}`)
}
