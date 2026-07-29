<template>
  <el-dialog :model-value="visible" title="物模型定义" width="760px" @update:model-value="$emit('update:visible', $event)">
    <el-tabs v-model="tab">
      <el-tab-pane label="属性" name="props">
        <el-table :data="properties" size="small" max-height="360">
          <el-table-column label="标识符" min-width="120">
            <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="temperature" /></template>
          </el-table-column>
          <el-table-column label="名称" min-width="100">
            <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="温度" /></template>
          </el-table-column>
          <el-table-column label="类型" width="100">
            <template #default="{ row }">
              <el-select v-model="row.dataType" size="small">
                <el-option v-for="t in ['int', 'float', 'bool', 'string']" :key="t" :label="t" :value="t" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="单位" width="80">
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
          <el-table-column width="60">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="properties.splice($index, 1)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-button style="margin-top: 10px" size="small" @click="addProperty">
          <el-icon><Plus /></el-icon>&nbsp;添加属性
        </el-button>
      </el-tab-pane>

      <el-tab-pane label="服务" name="services">
        <el-table :data="services" size="small" max-height="360">
          <el-table-column label="标识符" min-width="120">
            <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="reboot" /></template>
          </el-table-column>
          <el-table-column label="名称" min-width="100">
            <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="重启设备" /></template>
          </el-table-column>
          <el-table-column label="描述" min-width="140">
            <template #default="{ row }"><el-input v-model="row.desc" size="small" /></template>
          </el-table-column>
          <el-table-column width="60">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="services.splice($index, 1)">删除</el-button>
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
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type TslProperty, type TslService } from '../api'

const props = defineProps<{ visible: boolean; productId: number | null }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const tab = ref('props')
const saving = ref(false)
const properties = ref<TslProperty[]>([])
const services = ref<TslService[]>([])

watch(
  () => props.visible,
  async (v) => {
    if (v && props.productId) {
      const tm = await api.getThingModel(props.productId)
      properties.value = (typeof tm.properties === 'string' ? JSON.parse(tm.properties as any) : tm.properties) || []
      services.value = (typeof tm.services === 'string' ? JSON.parse(tm.services as any) : tm.services) || []
    }
  }
)

function addProperty() {
  properties.value.push({ identifier: '', name: '', dataType: 'float', unit: '', min: null, max: null, accessMode: 'r' })
}
function addService() {
  services.value.push({ identifier: '', name: '', desc: '', params: [] })
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
    await api.saveThingModel(props.productId!, { properties: properties.value, services: services.value })
    ElMessage.success('物模型已保存')
    emit('update:visible', false)
  } finally {
    saving.value = false
  }
}
</script>
