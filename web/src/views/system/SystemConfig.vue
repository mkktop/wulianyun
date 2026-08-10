<template>
  <el-card shadow="never">
    <div class="toolbar">
      <span class="desc">热更新参数修改后立即生效；基础设施参数（存储/MQTT/数据库等）只读展示，需改 config.yaml 并重启服务</span>
    </div>

    <!-- 热更新参数 -->
    <el-table :data="settings" v-loading="loading" stripe>
      <el-table-column label="参数" min-width="220">
        <template #default="{ row }">
          <span class="param-name">{{ settingNames[row.key] || row.key }}</span>
          <code class="key">{{ row.key }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="说明" min-width="240" />
      <el-table-column label="类型" width="90">
        <template #default="{ row }"><el-tag size="small">{{ typeNames[row.type] || row.type }}</el-tag></template>
      </el-table-column>
      <el-table-column label="当前值" min-width="160">
        <template #default="{ row }">
          <span v-if="row.value !== ''">{{ row.value }}</span>
          <el-tag v-else size="small" type="info">跟随配置文件</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="110">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openEdit(row)">修改</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 对象存储配置（可编辑并热生效） -->
    <el-card shadow="never" class="storage-card">
      <template #header>
        <div class="storage-head">
          <span class="group-title">对象存储</span>
          <el-tag v-if="storage.type === 's3'" type="warning" size="small">S3 兼容</el-tag>
          <el-tag v-else size="small">本地磁盘</el-tag>
          <span class="storage-hint">修改后立即生效（新上传固件走新配置，已存固件不受影响）</span>
        </div>
      </template>
      <el-form :model="storage" label-width="110px" class="storage-form">
        <el-form-item label="存储类型">
          <el-radio-group v-model="storage.type">
            <el-radio value="local">本地磁盘</el-radio>
            <el-radio value="s3">S3 兼容对象存储</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="storage.type === 'local'">
          <el-form-item label="本地目录">
            <el-input v-model="storage.localDir" placeholder="如 uploads" style="width: 280px" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="端点 Endpoint">
            <el-input v-model="storage.endpoint" placeholder="如 oss-cn-hangzhou.aliyuncs.com（不带协议）" />
          </el-form-item>
          <el-form-item label="区域 Region">
            <el-input v-model="storage.region" placeholder="AWS 必填；阿里 OSS 可留空" style="width: 280px" />
          </el-form-item>
          <el-form-item label="桶名 Bucket">
            <el-input v-model="storage.bucket" placeholder="需设为公开读" style="width: 280px" />
          </el-form-item>
          <el-form-item label="AccessKey">
            <el-input v-model="storage.accessKey" placeholder="AccessKey ID" style="width: 320px" />
          </el-form-item>
          <el-form-item label="SecretKey">
            <el-input
              v-model="storage.secretKey" type="password" show-password
              :placeholder="storage.hasSecretKey ? '已设置（留空表示不修改）' : '请输入 SecretKey'"
              style="width: 320px"
            />
          </el-form-item>
          <el-form-item label="启用 HTTPS">
            <el-switch v-model="storage.useSSL" />
            <span class="storage-warn">注意：廉价 4G 模组 TLS 兼容性，默认关闭</span>
          </el-form-item>
          <el-form-item label="公开域名">
            <el-input v-model="storage.publicDomain" placeholder="CDN/自定义域名，空则用 bucket.endpoint" />
          </el-form-item>
        </template>
        <el-form-item>
          <el-button type="primary" :loading="storageSaving" @click="saveStorage">保存并热重载</el-button>
          <el-button @click="loadStorage">重置</el-button>
          <span v-if="storage.updatedAt" class="storage-updated">上次修改：{{ storage.updatedAt }}</span>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 全部配置只读展示 -->
    <el-divider content-position="left">当前生效配置（只读，敏感项已打码）</el-divider>
    <el-collapse v-model="openSections">
      <el-collapse-item v-for="(group, name) in config" :key="name" :name="name">
        <template #title>
          <span class="group-title">{{ groupNames[name] || name }}</span>
          <code class="group-key">{{ name }}</code>
        </template>
        <el-descriptions :column="3" border size="small">
          <el-descriptions-item v-for="(v, k) in group" :key="String(k)" :label="fieldLabel(name, String(k))">
            {{ formatVal(v) }}
          </el-descriptions-item>
        </el-descriptions>
      </el-collapse-item>
    </el-collapse>

    <!-- 修改参数 -->
    <el-dialog v-model="editVisible" title="修改参数" width="440px" :close-on-click-modal="false">
      <el-form label-width="90px" @submit.prevent>
        <el-form-item label="参数">
          <span class="param-name">{{ settingNames[editForm.key] || editForm.key }}</span>
          <code class="key">{{ editForm.key }}</code>
        </el-form-item>
        <el-form-item :label="editForm.type === 'bool' ? '开关' : '数值'">
          <el-switch
            v-if="editForm.type === 'bool'"
            v-model="editForm.value"
            active-text="开启"
            inactive-text="关闭"
          />
          <el-input-number v-else v-model="editForm.value" :min="1" :max="8760" style="width: 100%" />
        </el-form-item>
        <el-form-item label="说明">
          <span class="desc">{{ editForm.description }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="doSave">保存</el-button>
      </template>
    </el-dialog>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type SystemSettingItem, type StorageConfig } from '../../api'

const settings = ref<SystemSettingItem[]>([])
const config = ref<Record<string, Record<string, unknown>>>({})
const loading = ref(false)
const openSections = ref<string[]>([])

// 对象存储配置（可编辑并热生效）
const storage = reactive({
  type: 'local', localDir: 'uploads', endpoint: '', region: '', bucket: '',
  accessKey: '', hasSecretKey: false, secretKey: '', useSSL: false, publicDomain: '', updatedAt: '',
})
const storageSaving = ref(false)
async function loadStorage() {
  try {
    const s = await api.admin.storage.get()
    Object.assign(storage, s, { secretKey: '' }) // secretKey 不回显，编辑框留空表示不改
  } catch { /* 错误已由拦截器提示 */ }
}
async function saveStorage() {
  storageSaving.value = true
  try {
    await api.admin.storage.update({
      type: storage.type, localDir: storage.localDir, endpoint: storage.endpoint,
      region: storage.region, bucket: storage.bucket, accessKey: storage.accessKey,
      secretKey: storage.secretKey, useSSL: storage.useSSL, publicDomain: storage.publicDomain,
    })
    ElMessage.success('已保存并热重载，新上传固件将走新配置')
    loadStorage()
  } catch { /* 错误已由拦截器提示 */ } finally {
    storageSaving.value = false
  }
}

// 热参数中文名
const settingNames: Record<string, string> = {
  register_enabled: '开放注册',
  jwt_expire_hours: '登录令牌有效期',
  trace_retention_days: '消息轨迹保留',
  device_log_retention_days: '设备日志保留',
}
const typeNames: Record<string, string> = { bool: '开关', int: '数值' }

// 配置段中文名
const groupNames: Record<string, string> = {
  server: '服务', gateway: 'TCP 网关', jwt: '认证令牌', database: '数据库',
  redis: '缓存', mqtt: 'MQTT', telemetry_buffer: '遥测缓冲', cache: '设备缓存',
  poller: '轮询引擎', log: '日志', emqx_rule: 'EMQX 规则', storage: '对象存储',
  admin_password_set: '管理员初始密码',
}

// 各配置段的字段中文名（key 格式：段名.字段名）
const fieldNames: Record<string, string> = {
  'server.addr': '监听地址',
  'gateway.addr': '监听地址', 'gateway.idle_timeout': '空闲超时(秒)', 'gateway.max_conns_per_ip': '单IP最大连接',
  'gateway.conn_rate_limit': '连接速率', 'gateway.conn_rate_burst': '速率桶容量', 'gateway.tls_enabled': '启用TLS',
  'jwt.expire_hours': '有效期(小时)', 'jwt.secret': '密钥',
  'database.dsn': '连接串', 'database.max_open_conns': '最大连接数', 'database.max_idle_conns': '最大空闲连接',
  'database.conn_max_lifetime': '连接最长存活(秒)', 'database.conn_max_idle_time': '空闲最长存活(秒)',
  'database.retention_days': '遥测保留(天)', 'database.compress_after_days': '压缩阈值(天)',
  'redis.addr': '地址', 'redis.db': '库', 'redis.password': '密码',
  'mqtt.broker': 'Broker', 'mqtt.client_id': '客户端ID', 'mqtt.username': '用户名', 'mqtt.password': '密码', 'mqtt.tls_enabled': '启用TLS',
  'telemetry_buffer.max_batch': '最大批次', 'telemetry_buffer.flush_interval': '刷新间隔(秒)',
  'cache.device_ttl': '设备缓存(秒)', 'cache.shadow_flush_interval': '影子刷新(秒)',
  'poller.max_concurrent': '最大并发',
  'log.trace_retention_days': '轨迹保留(天)', 'log.device_log_retention_days': '设备日志保留(天)',
  'emqx_rule.enabled': '已启用',
  'storage.type': '类型', 'storage.local_dir': '本地目录', 'storage.endpoint': '端点', 'storage.region': '区域',
  'storage.bucket': '桶名', 'storage.access_key': 'AccessKey', 'storage.secret_key': 'SecretKey',
  'storage.use_ssl': '启用SSL', 'storage.public_domain': '公开域名',
  'admin_password_set': '已设置',
}

function fieldLabel(group: string, field: string): string {
  return fieldNames[`${group}.${field}`] || fieldNames[field] || field
}

// 布尔值显示中文，其余原样
function formatVal(v: unknown): string {
  if (typeof v === 'boolean') return v ? '是' : '否'
  return v == null ? '' : String(v)
}

const editVisible = ref(false)
const saving = ref(false)
const editForm = reactive({ key: '', value: '', type: '', description: '' })

async function load() {
  loading.value = true
  try {
    const [s, c] = await Promise.all([api.admin.settings.list(), api.admin.systemConfig()])
    settings.value = s || []
    config.value = c || {}
    // 默认只展开运维最关心的几段，其余折叠（避免首屏过长）
    const defaultOpen = new Set(['server', 'gateway', 'storage', 'mqtt', 'jwt', 'database'])
    openSections.value = Object.keys(config.value).filter((k) => defaultOpen.has(k))
  } catch { /* 错误已由拦截器提示 */ } finally {
    loading.value = false
  }
}

function openEdit(row: SystemSettingItem) {
  editForm.key = row.key
  editForm.type = row.type
  editForm.description = row.description
  editForm.value = row.type === 'bool' ? (row.value !== '' ? row.value : 'true') : (row.value || '')
  editVisible.value = true
}

async function doSave() {
  saving.value = true
  try {
    // bool 转成字符串提交；空值表示恢复默认
    const value = editForm.type === 'bool' ? (editForm.value === 'true' ? 'true' : 'false') : String(editForm.value)
    await api.admin.settings.update({ key: editForm.key, value })
    ElMessage.success('已保存并立即生效')
    editVisible.value = false
    load()
  } catch { /* 错误已由拦截器提示 */ } finally {
    saving.value = false
  }
}

onMounted(() => {
  load()
  loadStorage()
})
</script>

<style scoped>
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.desc { color: #909399; font-size: 13px; }
.key { font-family: Consolas, monospace; color: #909399; font-size: 12px; margin-left: 8px; }
.param-name { color: #303133; font-weight: 500; }
.group-title { color: #303133; font-weight: 600; }
.group-key { font-family: Consolas, monospace; color: #c0c4cc; font-size: 12px; margin-left: 8px; }
.storage-card { margin: 16px 0; }
.storage-head { display: flex; align-items: center; gap: 8px; }
.storage-hint { color: #909399; font-size: 12px; margin-left: auto; }
.storage-form { margin-top: 8px; }
.storage-warn { color: #e6a23c; font-size: 12px; margin-left: 8px; }
.storage-updated { color: #c0c4cc; font-size: 12px; margin-left: 12px; }
</style>
