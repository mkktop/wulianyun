<template>
  <el-dialog :model-value="visible" title="物模型定义 (TSL)" width="860px" @update:model-value="$emit('update:visible', $event)">
    <el-tabs v-model="tab">
      <!-- 属性 -->
      <el-tab-pane label="属性" name="props">
        <el-table :data="properties" size="small" max-height="380">
          <el-table-column label="标识符" min-width="120">
            <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="temperature" /></template>
          </el-table-column>
          <el-table-column label="名称" min-width="90">
            <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="温度" /></template>
          </el-table-column>
          <el-table-column label="数据类型" width="110">
            <template #default="{ row }">
              <el-select v-model="row.dataType" size="small">
                <el-option v-for="t in dataTypes" :key="t" :label="t" :value="t" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="取值范围" width="150">
            <template #default="{ row }">
              <div v-if="isNumeric(row.dataType)" class="range">
                <el-input v-model.number="row.min" size="small" placeholder="min" />
                <span>~</span>
                <el-input v-model.number="row.max" size="small" placeholder="max" />
              </div>
              <el-button v-else-if="row.dataType === 'enum'" link type="primary" size="small" @click="editEnum(row)">
                枚举项({{ (row.enumSpec || []).length }})
              </el-button>
              <span v-else class="muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="步长" width="70">
            <template #default="{ row }">
              <el-input v-if="isNumeric(row.dataType)" v-model.number="row.step" size="small" placeholder="1" />
              <span v-else class="muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="单位" width="70">
            <template #default="{ row }"><el-input v-model="row.unit" size="small" placeholder="℃" /></template>
          </el-table-column>
          <el-table-column label="读写" width="90">
            <template #default="{ row }">
              <el-select v-model="row.accessMode" size="small">
                <el-option label="只读" value="r" />
                <el-option label="读写" value="rw" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column width="50">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="properties.splice($index, 1)">删</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-button style="margin-top: 10px" size="small" @click="addProperty">
          <el-icon><Plus /></el-icon>&nbsp;添加属性
        </el-button>
      </el-tab-pane>

      <!-- 事件 -->
      <el-tab-pane label="事件" name="events">
        <el-table :data="events" size="small" max-height="380">
          <el-table-column label="标识符" min-width="120">
            <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="highTemp" /></template>
          </el-table-column>
          <el-table-column label="名称" min-width="110">
            <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="高温告警" /></template>
          </el-table-column>
          <el-table-column label="类型" width="120">
            <template #default="{ row }">
              <el-select v-model="row.type" size="small">
                <el-option label="信息" value="info" />
                <el-option label="告警" value="alert" />
                <el-option label="故障" value="fault" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="描述" min-width="140">
            <template #default="{ row }"><el-input v-model="row.desc" size="small" /></template>
          </el-table-column>
          <el-table-column width="50">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="events.splice($index, 1)">删</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-button style="margin-top: 10px" size="small" @click="addEvent">
          <el-icon><Plus /></el-icon>&nbsp;添加事件
        </el-button>
      </el-tab-pane>

      <!-- 服务 -->
      <el-tab-pane label="服务" name="services">
        <el-table :data="services" size="small" max-height="380">
          <el-table-column label="标识符" min-width="120">
            <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="reboot" /></template>
          </el-table-column>
          <el-table-column label="名称" min-width="110">
            <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="重启设备" /></template>
          </el-table-column>
          <el-table-column label="调用方式" width="110">
            <template #default="{ row }">
              <el-select v-model="row.async" size="small">
                <el-option label="同步" :value="false" />
                <el-option label="异步" :value="true" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="描述" min-width="140">
            <template #default="{ row }"><el-input v-model="row.desc" size="small" /></template>
          </el-table-column>
          <el-table-column width="50">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="services.splice($index, 1)">删</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-button style="margin-top: 10px" size="small" @click="addService">
          <el-icon><Plus /></el-icon>&nbsp;添加服务
        </el-button>
      </el-tab-pane>
    </el-tabs>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>

  <!-- 枚举项编辑 -->
  <el-dialog v-model="enumVisible" title="枚举项定义" width="420px" append-to-body>
    <div v-for="(it, i) in enumEditing" :key="i" class="enum-row">
      <el-input v-model.number="it.value" placeholder="值(数字)" style="width: 120px" />
      <el-input v-model="it.label" placeholder="描述，如 制冷" style="flex: 1" />
      <el-button link type="danger" @click="enumEditing.splice(i, 1)">删</el-button>
    </div>
    <el-button size="small" @click="enumEditing.push({ value: 0, label: '' })">
      <el-icon><Plus /></el-icon>&nbsp;添加枚举项
    </el-button>
    <template #footer>
      <el-button @click="enumVisible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type TslProperty, type TslEvent, type TslService, type TslEnumItem } from '../api'

const props = defineProps<{ visible: boolean; productId: number | null }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const dataTypes = ['int32', 'float', 'double', 'bool', 'enum', 'text', 'date']
const tab = ref('props')
const saving = ref(false)
const properties = ref<TslProperty[]>([])
const events = ref<TslEvent[]>([])
const services = ref<TslService[]>([])

const enumVisible = ref(false)
const enumEditing = ref<TslEnumItem[]>([])
let enumTarget: TslProperty | null = null

function isNumeric(t: string) {
  return t === 'int32' || t === 'float' || t === 'double'
}

function toArr(v: any) {
  if (!v) return []
  return typeof v === 'string' ? JSON.parse(v) : v
}

watch(
  () => props.visible,
  async (v) => {
    if (v && props.productId) {
      const tm = await api.getThingModel(props.productId)
      properties.value = toArr(tm.properties)
      events.value = toArr(tm.events)
      services.value = toArr(tm.services)
      tab.value = 'props'
    }
  }
)

function addProperty() {
  properties.value.push({
    identifier: '', name: '', dataType: 'float', unit: '',
    min: null, max: null, step: null, accessMode: 'r', enumSpec: [], desc: ''
  })
}
function addEvent() {
  events.value.push({ identifier: '', name: '', type: 'info', outputs: [], desc: '' })
}
function addService() {
  services.value.push({ identifier: '', name: '', async: false, inputs: [], outputs: [], desc: '' })
}

function editEnum(row: TslProperty) {
  enumTarget = row
  if (!row.enumSpec) row.enumSpec = []
  enumEditing.value = row.enumSpec
  enumVisible.value = true
}

async function save() {
  for (const p of properties.value) {
    if (!p.identifier || !p.name) {
      ElMessage.warning('属性的标识符和名称必填')
      return
    }
  }
  saving.value = true
  try {
    await api.saveThingModel(props.productId!, {
      properties: properties.value, events: events.value, services: services.value
    })
    ElMessage.success('物模型已保存')
    emit('update:visible', false)
  } catch (e: any) {
    // 后端校验错误已由拦截器提示
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.range { display: flex; align-items: center; gap: 4px; }
.muted { color: #ccc; }
.enum-row { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; }
</style>
