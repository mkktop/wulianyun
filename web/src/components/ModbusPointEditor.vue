<template>
  <div>
    <el-alert type="info" :closable="false" style="margin-bottom: 12px">
      平台按<b>采集组</b>的独立周期轮询，同组内地址连续的寄存器会<b>合并成一次请求</b>（大幅降低总线往返）。变更上报模式下仅上报有变化的点位。
    </el-alert>

    <!-- 采集组管理（需产品已创建） -->
    <div class="group-bar" v-if="productId">
      <span class="bar-label">采集组：</span>
      <el-tag
        :type="activeGroup === 0 ? 'primary' : 'info'"
        :effect="activeGroup === 0 ? 'dark' : 'plain'"
        class="group-tag" @click="activeGroup = 0"
      >默认组（产品周期）</el-tag>
      <el-tag
        v-for="g in groups" :key="g.id"
        :type="activeGroup === g.id ? 'primary' : 'info'"
        :effect="activeGroup === g.id ? 'dark' : 'plain'"
        class="group-tag" closable @click="activeGroup = g.id" @close="removeGroup(g)"
      >{{ g.name }}（{{ g.pollInterval }}s·{{ g.reportMode === 'onchange' ? '变更' : '周期' }}）</el-tag>
      <el-button size="small" @click="groupDialogVisible = true">
        <el-icon><Plus /></el-icon>&nbsp;新建组
      </el-button>
    </div>
    <el-alert v-else type="warning" :closable="false" style="margin-bottom: 12px">
      采集分组需先保存产品后再配置；当前点位默认归入「默认组」，按产品采集周期轮询。
    </el-alert>

    <el-table :data="visiblePoints" size="small" max-height="420">
      <el-table-column label="标识符" min-width="110">
        <template #default="{ row }"><el-input v-model="row.identifier" size="small" placeholder="temperature" /></template>
      </el-table-column>
      <el-table-column label="名称" min-width="90">
        <template #default="{ row }"><el-input v-model="row.name" size="small" placeholder="温度" /></template>
      </el-table-column>
      <el-table-column v-if="productId" label="采集组" width="130">
        <template #default="{ row }">
          <el-select v-model="row.groupId" size="small">
            <el-option :value="0" label="默认组" />
            <el-option v-for="g in groups" :key="g.id" :label="g.name" :value="g.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="从机" width="66">
        <template #default="{ row }"><el-input v-model.number="row.slaveId" size="small" /></template>
      </el-table-column>
      <el-table-column label="功能码" width="140">
        <template #default="{ row }">
          <el-select v-model="row.functionCode" size="small">
            <el-option v-for="f in funcCodes" :key="f.code" :label="f.label" :value="f.code" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="地址" width="76">
        <template #default="{ row }"><el-input v-model.number="row.address" size="small" /></template>
      </el-table-column>
      <el-table-column label="类型" width="96">
        <template #default="{ row }">
          <el-select v-model="row.rawType" size="small">
            <el-option v-for="t in rawTypes" :key="t" :label="t" :value="t" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="缩放" width="66">
        <template #default="{ row }"><el-input v-model.number="row.scale" size="small" /></template>
      </el-table-column>
      <el-table-column label="字节序" width="110">
        <template #default="{ row }">
          <el-checkbox v-model="row.swapByte" size="small">字节</el-checkbox>
          <el-checkbox v-model="row.swapWord" size="small">字</el-checkbox>
        </template>
      </el-table-column>
      <el-table-column label="读写" width="86">
        <template #default="{ row }">
          <el-select v-model="row.accessMode" size="small">
            <el-option label="只读" value="r" />
            <el-option label="读写" value="rw" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="单位" width="64">
        <template #default="{ row }"><el-input v-model="row.unit" size="small" /></template>
      </el-table-column>
      <el-table-column width="46" fixed="right">
        <template #default="{ row }">
          <el-button link type="danger" size="small" @click="removePoint(row)">删</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-button style="margin-top: 10px" size="small" @click="addPoint">
      <el-icon><Plus /></el-icon>&nbsp;添加点位{{ productId && activeGroup ? '（到当前组）' : '' }}
    </el-button>

    <template v-if="productId">
      <el-divider />
      <div class="test-row">
        <span>解析测试：</span>
        <el-input v-model="testHex" placeholder="应答帧 hex，如 01 03 02 00FA CRC" style="flex: 1" />
        <el-select v-model="testIndex" placeholder="选择点位" style="width: 160px">
          <el-option v-for="(pt, i) in points" :key="i" :label="pt.name || pt.identifier || `点位${i + 1}`" :value="i" />
        </el-select>
        <el-button @click="test" :loading="testing">测试</el-button>
      </div>
      <el-alert v-if="testResult" :type="testOk ? 'success' : 'error'" :closable="false" style="margin-top: 8px">
        {{ testResult }}
      </el-alert>
    </template>

    <!-- 新建采集组 -->
    <el-dialog v-model="groupDialogVisible" title="新建采集组" width="420px" append-to-body>
      <el-form label-width="90px">
        <el-form-item label="组名称" required>
          <el-input v-model="groupForm.name" placeholder="如：高频组" />
        </el-form-item>
        <el-form-item label="采集周期">
          <el-input-number v-model="groupForm.pollInterval" :min="1" /> &nbsp;秒
        </el-form-item>
        <el-form-item label="上报方式">
          <el-radio-group v-model="groupForm.reportMode">
            <el-radio value="periodic">按周期</el-radio>
            <el-radio value="onchange">变更上报</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createGroup">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api, type ModbusPoint, type ModbusGroup } from '../api'

const props = defineProps<{ productId?: number | null }>()
const points = defineModel<ModbusPoint[]>('points', { default: () => [] })

const funcCodes = [
  { code: 1, label: '01 读线圈' },
  { code: 2, label: '02 读离散输入' },
  { code: 3, label: '03 读保持寄存器' },
  { code: 4, label: '04 读输入寄存器' }
]
const rawTypes = ['int16', 'uint16', 'int32', 'uint32', 'float', 'bool', 'bits']

const groups = ref<ModbusGroup[]>([])
const activeGroup = ref(0) // 当前查看的组（0=默认组）
const groupDialogVisible = ref(false)
const groupForm = reactive({ name: '', pollInterval: 10, reportMode: 'periodic' })

const testHex = ref('')
const testIndex = ref(0)
const testResult = ref('')
const testOk = ref(false)
const testing = ref(false)

// 按当前选中组过滤显示点位
const visiblePoints = computed(() => points.value.filter((p) => (p.groupId || 0) === activeGroup.value))

async function loadGroups() {
  if (!props.productId) return
  groups.value = await api.listModbusGroups(props.productId)
}

watch(() => props.productId, loadGroups, { immediate: true })

async function createGroup() {
  if (!groupForm.name) {
    ElMessage.warning('请输入组名称')
    return
  }
  const g = await api.createModbusGroup(props.productId!, groupForm)
  ElMessage.success('采集组已创建')
  groupDialogVisible.value = false
  groupForm.name = ''
  await loadGroups()
  activeGroup.value = g.id
}

async function removeGroup(g: ModbusGroup) {
  await api.deleteModbusGroup(props.productId!, g.id)
  ElMessage.success('已删除，组内点位归入默认组')
  points.value.forEach((p) => { if (p.groupId === g.id) p.groupId = 0 })
  if (activeGroup.value === g.id) activeGroup.value = 0
  await loadGroups()
}

function addPoint() {
  points.value.push({
    identifier: '', name: '', groupId: props.productId ? activeGroup.value : 0,
    slaveId: 1, functionCode: 3, address: 0,
    rawType: 'uint16', bitPosition: 0, scale: 1, offset: 0,
    swapByte: false, swapWord: false, accessMode: 'r', unit: ''
  })
}

function removePoint(row: ModbusPoint) {
  const i = points.value.indexOf(row)
  if (i >= 0) points.value.splice(i, 1)
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
.group-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
.bar-label { color: #606266; font-size: 13px; }
.group-tag { cursor: pointer; }
.test-row { display: flex; align-items: center; gap: 8px; }
</style>
