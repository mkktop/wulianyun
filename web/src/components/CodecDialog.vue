<template>
  <el-dialog :model-value="visible" title="协议解析脚本" width="720px" @update:model-value="$emit('update:visible', $event)">
    <el-alert type="info" :closable="false" style="margin-bottom: 12px">
      <p>适用于 TCP 透传设备。脚本包含 <code>decode(bytes)</code> 上行解码（必须），<code>encode(obj)</code> 下行编码（可选）。</p>
      <p>bytes 为字节数组（0-255 整数），decode 需返回属性对象；清空脚本保存即关闭脚本解析。</p>
    </el-alert>
    <el-input
      v-model="script" type="textarea" :rows="12" spellcheck="false"
      style="font-family: Consolas, monospace"
      placeholder="function decode(bytes) {
  return {
    temperature: ((bytes[0] << 8) | bytes[1]) / 10,
    humidity: ((bytes[2] << 8) | bytes[3]) / 10
  }
}"
    />
    <div class="test-row">
      <el-input v-model="testHex" placeholder="测试报文 hex，如 00FA 0223" style="flex: 1" />
      <el-button @click="test" :loading="testing">测试解析</el-button>
    </div>
    <el-alert v-if="testResult" :type="testOk ? 'success' : 'error'" :closable="false" style="margin-top: 8px">
      {{ testResult }}
    </el-alert>
    <template #footer>
      <el-button @click="$emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="saving" @click="save">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api'

const props = defineProps<{ visible: boolean; productId: number | null }>()
const emit = defineEmits<{ (e: 'update:visible', v: boolean): void }>()

const script = ref('')
const testHex = ref('')
const testResult = ref('')
const testOk = ref(false)
const testing = ref(false)
const saving = ref(false)

watch(
  () => props.visible,
  async (v) => {
    if (v && props.productId) {
      const res = await api.getCodec(props.productId)
      script.value = res.script || ''
      testResult.value = ''
    }
  }
)

async function test() {
  if (!script.value || !testHex.value) {
    ElMessage.warning('请输入脚本和测试报文')
    return
  }
  testing.value = true
  try {
    const res = await api.testCodec(props.productId!, script.value, testHex.value)
    testResult.value = '解析结果: ' + JSON.stringify(res)
    testOk.value = true
  } catch (e: any) {
    testResult.value = '解析失败: ' + (e.message || e)
    testOk.value = false
  } finally {
    testing.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await api.saveCodec(props.productId!, script.value)
    ElMessage.success('脚本已保存')
    emit('update:visible', false)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.test-row { display: flex; gap: 8px; margin-top: 12px; }
</style>
