<template>
  <div>
    <el-alert type="info" :closable="false" style="margin-bottom: 12px">
      平台按产品采集周期在 DTU 长连接上轮询以下点位（读功能码 1-4）；读写点位(rw)支持下发控制。缩放公式：业务值 = 原始值 × 缩放 + 偏移。
    </el-alert>
    <el-table :data="points" size="small" max-height="420">
      <el-table-column label="标识符" min-width="110">
        <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="temperature" /></template>
      </el-table-column>
      <el-table-column label="名称" min-width="90">
        <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="温度" /></template>
      </el-table-column>
      <el-table-column label="从机" width="70">
        <template #default="{ row }"><el-input v-model.number="row.slaveId" size="small" placeholder="1" /></template>
      </el-table-column>
      <el-table-column label="功能码" width="150">
        <template #default="{ row }">
          <el-select v-model="row.functionCode" size="small">
            <el-option v-for="f in funcCodes" :key="f.code" :label="f.label" :value="f.code" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="地址" width="80">
        <template #default="{ row }"><el-input v-model.number="row.address" size="small" placeholder="0" /></template>
      </el-table-column>
      <el-table-column label="数据类型" width="100">
        <template #default="{ row }">
          <el-select v-model="row.rawType" size="small">
            <el-option v-for="t in rawTypes" :key="t" :label="t" :value="t" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="缩放" width="70">
        <template #default="{ row }"><el-input v-model.number="row.scale" size="small" placeholder="1" /></template>
      </el-table-column>
      <el-table-column label="字节序" width="120">
        <template #default="{ row }">
          <el-checkbox v-model="row.swapByte" size="small">字节</el-checkbox>
          <el-checkbox v-model="row.swapWord" size="small">字</el-checkbox>
        </template>
      </el-table-column>
      <el-table-column label="读写" width="90">
        <template #default="{ row }">
          <el-select v-model="row.accessMode" size="small">
            <el-option label="只读" value="r" />
            <el-option label="读写" value="rw" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="单位" width="70">
        <template #default="{ row }"><el-input v-model="row.unit" size="small" /></template>
      </el-table-column>
      <el-table-column width="50" fixed="right">
        <template #default="{ $index }">
          <el-button link type="danger" size="small" @click="points.splice($index, 1)">删</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-button style="margin-top: 10px" size="small" @click="addPoint">
      <el-icon><Plus /></el-icon>&nbsp;添加点位
    </el-button>

    <template v-if="productId">
      <el-divider />
      <div class="test-row">
        <span>解析测试：</span>
        <el-input v-model="testHex" placeholder="应答帧 hex，如 01 03 02 00FA CRC" style="flex: 1" />
        <el-select v-model="testIndex" placeholder="选择点位" style="width: 160px">
          <el-option v-for="(p, i) in points" :key="i" :label="p.name || p.identifier || `点位${i + 1}`" :value="i" />
        </el-select>
        <el-button @click="test" :loading="testing">测试</el-button>
      </div>
      <el-alert v-if="testResult" :type="testOk ? 'success' : 'error'" :closable="false" style="margin-top: 8px">
        {{ testResult }}
      </el-alert>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type ModbusPoint } from '../api'

const props = defineProps<{ productId?: number | null }>()
const points = defineModel<ModbusPoint[]>('points', { default: () => [] })

const funcCodes = [
  { code: 1, label: '01 读线圈' },
  { code: 2, label: '02 读离散输入' },
  { code: 3, label: '03 读保持寄存器' },
  { code: 4, label: '04 读输入寄存器' }
]
const rawTypes = ['int16', 'uint16', 'int32', 'uint32', 'float', 'bool', 'bits']

const testHex = ref('')
const testIndex = ref(0)
const testResult = ref('')
const testOk = ref(false)
const testing = ref(false)

function addPoint() {
  points.value.push({
    identifier: '', name: '', slaveId: 1, functionCode: 3, address: 0,
    rawType: 'uint16', bitPosition: 0, scale: 1, offset: 0,
    swapByte: false, swapWord: false, accessMode: 'r', unit: ''
  })
}

async function test() {
  const pt = points.value[testIndex.value]
  if (!pt || !testHex.value) {
    ElMessage.warning('请选择点位并输入应答帧')
    return
  }
  testing.value = true
  try {
    const res = await api.testModbusPoint(props.productId!, pt, testHex.value)
    testResult.value = `解析结果: ${pt.name || pt.identifier} = ${res.value}${pt.unit || ''}`
    testOk.value = true
  } catch (e: any) {
    testResult.value = '解析失败: ' + (e.message || e)
    testOk.value = false
  } finally {
    testing.value = false
  }
}
</script>

<style scoped>
.test-row { display: flex; align-items: center; gap: 8px; }
</style>
