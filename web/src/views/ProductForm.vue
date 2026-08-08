<template>
  <div>
    <el-page-header :content="isEdit ? '编辑产品' : '创建产品'" @back="$router.push('/products')" style="margin-bottom: 16px" />

    <!-- 基本信息 -->
    <el-card shadow="never">
      <template #header>基本信息</template>
      <el-form :model="form" label-width="100px" style="max-width: 640px">
        <el-form-item label="产品名称" required>
          <el-input v-model="form.name" placeholder="如：温湿度传感器" />
        </el-form-item>
        <el-form-item label="接入协议">
          <el-radio-group v-model="form.protocol" :disabled="isEdit" @change="onProtocolChange">
            <el-radio value="mqtt">MQTT</el-radio>
            <el-radio value="tcp">TCP透传</el-radio>
            <el-radio value="http">HTTP</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="接入方式">
          <el-radio-group v-model="form.accessMode" :disabled="isEdit">
            <el-radio value="thingmodel">物模型</el-radio>
            <el-radio value="passthrough">脚本解析</el-radio>
            <el-radio value="modbus" :disabled="form.protocol !== 'tcp'">Modbus点表</el-radio>
          </el-radio-group>
          <div class="hint">
            {{ accessHint }}
            <span v-if="form.protocol !== 'tcp'" class="warn">（Modbus 仅支持 TCP 协议）</span>
          </div>
        </el-form-item>
        <el-form-item label="采集周期" v-if="form.accessMode === 'modbus'">
          <el-input-number v-model="form.pollInterval" :min="60" :step="60" /> &nbsp;秒（最小 60）
        </el-form-item>
        <el-form-item label="密钥模式">
          <el-radio-group v-model="form.secretMode" :disabled="isEdit">
            <el-radio value="device">一机一密</el-radio>
            <el-radio value="product">一型一密</el-radio>
          </el-radio-group>
          <div class="hint">{{ form.secretMode === 'product' ? '产品共用密钥，设备首次连接自动注册' : '每个设备独立密钥，安全性高' }}</div>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
    </el-card>

    <!-- TCP 组帧与心跳（透传/脚本解析产品；Modbus 固定按 RTU 帧组帧） -->
    <el-card v-if="form.protocol === 'tcp' && form.accessMode !== 'modbus'" shadow="never" style="margin-top: 16px">
      <template #header>TCP 组帧与心跳</template>
      <el-form :model="form" label-width="100px" style="max-width: 640px">
        <el-form-item label="组帧方式">
          <el-radio-group v-model="form.frameMode">
            <el-radio value="none">不组帧</el-radio>
            <el-radio value="delimiter">定界符</el-radio>
            <el-radio value="length">长度字段</el-radio>
          </el-radio-group>
          <div class="hint">{{ frameHint }}</div>
        </el-form-item>
        <el-form-item label="定界符" v-if="form.frameMode === 'delimiter'">
          <el-input v-model="form.frameDelimiter" placeholder="HEX，如 0D0A 表示 \r\n" style="width: 220px" />
        </el-form-item>
        <template v-if="form.frameMode === 'length'">
          <el-form-item label="长度字段">
            偏移 <el-input-number v-model="form.frameLenOffset" :min="0" :max="64" style="width: 110px" />
            &nbsp;字节数 <el-radio-group v-model="form.frameLenSize" style="margin-left: 6px">
              <el-radio :value="1">1</el-radio>
              <el-radio :value="2">2(大端)</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="长度调整">
            <el-input-number v-model="form.frameLenAdjust" :min="-64" :max="64" />
            <div class="hint">帧总长 = 长度字段值 + 调整值（如长度不含头部时补正）</div>
          </el-form-item>
        </template>
        <el-form-item label="心跳包">
          <el-input v-model="form.heartbeatPacket" placeholder="留空默认 PING；支持文本或 0x 开头 HEX" />
        </el-form-item>
        <el-form-item label="心跳回复">
          <el-input v-model="form.heartbeatReply" placeholder="留空不回复（默认心跳时回复 PONG）" />
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 功能定义：随接入方式切换 -->
    <el-card shadow="never" style="margin-top: 16px">
      <template #header>
        <span v-if="form.accessMode === 'thingmodel'">物模型定义</span>
        <span v-else-if="form.accessMode === 'modbus'">Modbus 点位表</span>
        <span v-else>协议解析脚本</span>
      </template>
      <ThingModelEditor
        v-if="form.accessMode === 'thingmodel'"
        v-model:properties="tsl.properties" v-model:events="tsl.events" v-model:services="tsl.services"
      />
      <ModbusPointEditor
        v-else-if="form.accessMode === 'modbus'"
        v-model:points="points" :product-id="productId"
      />
      <CodecEditor v-else v-model:script="script" :product-id="productId" />
    </el-card>

    <div class="footer-bar">
      <el-button @click="$router.push('/products')">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">{{ isEdit ? '保存' : '创建产品' }}</el-button>
    </div>

    <!-- 一型一密创建成功：展示产品密钥 -->
    <el-dialog v-model="secretDialogVisible" title="产品创建成功" width="460px" :close-on-click-modal="false" @close="goList">
      <el-alert type="success" :closable="false" style="margin-bottom: 12px">请妥善保存产品密钥，用于设备一型一密接入</el-alert>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="ProductID">{{ createdProduct?.productId }}</el-descriptions-item>
        <el-descriptions-item label="ProductSecret">
          <el-text>{{ createdProduct?.productSecret }}</el-text>
          <el-button link type="primary" size="small" @click="copyText(createdProduct?.productSecret || '')">复制</el-button>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="goList">我已保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { api, type Product, type ModbusPoint, type TslProperty, type TslEvent, type TslService } from '../api'
import ThingModelEditor from '../components/ThingModelEditor.vue'
import ModbusPointEditor from '../components/ModbusPointEditor.vue'
import CodecEditor from '../components/CodecEditor.vue'
import { copyText } from '../utils/clipboard'

const route = useRoute()
const router = useRouter()
const productId = ref<number | null>(route.params.id ? Number(route.params.id) : null)
const isEdit = computed(() => productId.value !== null)
const saving = ref(false)
const secretDialogVisible = ref(false)
const createdProduct = ref<Product | null>(null)

const form = reactive({
  name: '', protocol: 'mqtt', accessMode: 'thingmodel',
  secretMode: 'device', pollInterval: 60, description: '',
  frameMode: 'none', frameDelimiter: '', frameLenOffset: 0, frameLenSize: 1, frameLenAdjust: 0,
  heartbeatPacket: '', heartbeatReply: ''
})
const tsl = reactive<{ properties: TslProperty[]; events: TslEvent[]; services: TslService[] }>({
  properties: [], events: [], services: []
})
const points = ref<ModbusPoint[]>([])
const script = ref('')

const accessHint = computed(() => ({
  thingmodel: '设备上报标准 JSON，按物模型属性解析',
  passthrough: '设备上报自定义报文，用 JS 脚本解析',
  modbus: 'DTU 主动接入，平台按采集周期轮询 Modbus 点位'
} as any)[form.accessMode])

const frameHint = computed(() => ({
  none: '每次 TCP 读取视为一帧（报文较短且发送间隔大时可用；公网环境建议配置组帧）',
  delimiter: '按定界符切分数据帧，解决 TCP 粘包/拆包',
  length: '按报文内长度字段切分数据帧，解决 TCP 粘包/拆包'
} as any)[form.frameMode])

// 协议切换时：非 TCP 不允许 Modbus，自动回退到物模型
function onProtocolChange() {
  if (form.protocol !== 'tcp' && form.accessMode === 'modbus') {
    form.accessMode = 'thingmodel'
  }
}

function toArr(v: any) {
  if (!v) return []
  return typeof v === 'string' ? JSON.parse(v) : v
}

async function loadForEdit() {
  const p = await api.getProduct(productId.value!)
  form.name = p.name
  form.protocol = p.protocol
  form.accessMode = p.accessMode
  form.secretMode = p.secretMode
  form.pollInterval = p.pollInterval || 60
  form.description = p.description
  form.frameMode = p.frameMode || 'none'
  form.frameDelimiter = p.frameDelimiter || ''
  form.frameLenOffset = p.frameLenOffset || 0
  form.frameLenSize = p.frameLenSize || 1
  form.frameLenAdjust = p.frameLenAdjust || 0
  form.heartbeatPacket = p.heartbeatPacket || ''
  form.heartbeatReply = p.heartbeatReply || ''
  // 载入对应功能定义
  if (p.accessMode === 'thingmodel') {
    const tm = await api.getThingModel(p.id)
    tsl.properties = toArr(tm.properties)
    tsl.events = toArr(tm.events)
    tsl.services = toArr(tm.services)
  } else if (p.accessMode === 'modbus') {
    points.value = await api.getModbusPoints(p.id)
  } else {
    const c = await api.getCodec(p.id)
    script.value = c.script || ''
  }
}

// 保存功能定义（产品已存在）
async function saveDefinition(pid: number) {
  if (form.accessMode === 'thingmodel') {
    await api.saveThingModel(pid, { properties: tsl.properties, events: tsl.events, services: tsl.services })
  } else if (form.accessMode === 'modbus') {
    await api.saveModbusPoints(pid, points.value)
  } else {
    await api.saveCodec(pid, script.value)
  }
}

async function save() {
  if (!form.name) {
    ElMessage.warning('请输入产品名称')
    return
  }
  // 前端约束兜底
  if (form.accessMode === 'modbus' && form.protocol !== 'tcp') {
    ElMessage.warning('Modbus 仅支持 TCP 协议')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await api.updateProduct(productId.value!, form)
      await saveDefinition(productId.value!)
      ElMessage.success('保存成功')
      goList()
    } else {
      const p = await api.createProduct(form)
      await saveDefinition(p.id)
      if (p.secretMode === 'product' && p.productSecret) {
        createdProduct.value = p
        secretDialogVisible.value = true // 展示密钥后再跳转
      } else {
        ElMessage.success('创建成功')
        goList()
      }
    }
  } finally {
    saving.value = false
  }
}

function goList() {
  router.push('/products')
}


onMounted(() => {
  if (isEdit.value) loadForEdit()
})
</script>

<style scoped>
.hint { color: #999; font-size: 12px; margin-top: 4px; line-height: 1.4; }
.warn { color: #e6a23c; }
.footer-bar { margin-top: 16px; text-align: right; }
</style>
