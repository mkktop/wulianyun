// 设备模拟器：模拟一个 MQTT 设备连接平台并周期上报温湿度
// 用法: node simulator.js <productKey> <deviceName> <deviceSecret> [broker]
// 依赖: npm install mqtt （在本目录执行）
const mqtt = require('mqtt')

const [, , productKey, deviceName, secret, broker = 'mqtt://127.0.0.1:1883'] = process.argv
if (!productKey || !deviceName || !secret) {
  console.log('用法: node simulator.js <productKey> <deviceName> <deviceSecret> [broker]')
  process.exit(1)
}

const clientId = `${productKey}.${deviceName}`
const upTopic = `thing/up/${productKey}/${deviceName}`
const downTopic = `thing/down/${productKey}/${deviceName}`

const client = mqtt.connect(broker, {
  clientId,
  username: deviceName,
  password: secret,
  reconnectPeriod: 3000
})

// 可被平台下发修改的属性（影子闭环）
const state = { switch: 0, mode: 'normal' }

client.on('connect', () => {
  console.log(`[${deviceName}] 已连接 ${broker}`)
  client.subscribe(downTopic, () => console.log(`[${deviceName}] 已订阅下行主题 ${downTopic}`))
  let cycle = 0
  setInterval(() => {
    const payload = JSON.stringify({
      temperature: +(20 + Math.random() * 10).toFixed(2),
      humidity: +(40 + Math.random() * 30).toFixed(2),
      voltage: +(3.2 + Math.random() * 0.6).toFixed(3),
      ...state
    })
    client.publish(upTopic, payload, { qos: 1 })
    console.log(`[${deviceName}] 上报: ${payload}`)
    // 每 6 个周期上报一次事件（高温告警）
    if (++cycle % 6 === 0) {
      const evt = JSON.stringify({
        method: 'event.post', identifier: 'highTemp', type: 'alert',
        params: { temperature: +(35 + Math.random() * 5).toFixed(2) }
      })
      client.publish(upTopic, evt, { qos: 1 })
      console.log(`[${deviceName}] 事件上报: ${evt}`)
    }
  }, 5000)
})

client.on('message', (topic, message) => {
  console.log(`[${deviceName}] 收到下行 ${topic}: ${message.toString()}`)
  try {
    const msg = JSON.parse(message.toString())
    // 属性设置：更新本地状态并立即上报新值（平台据此清除影子 desired）
    if (msg.method === 'property.set' && msg.params) {
      Object.assign(state, msg.params)
      const report = JSON.stringify(msg.params)
      client.publish(upTopic, report, { qos: 1 })
      console.log(`[${deviceName}] 属性已生效并回报: ${report}`)
    }
    // 服务调用：模拟执行
    if (msg.method === 'service.invoke') {
      console.log(`[${deviceName}] 执行服务 ${msg.service}，参数:`, msg.params || {})
    }
  } catch { /* 非 JSON 忽略 */ }
})

client.on('error', (err) => console.error(`[${deviceName}] 错误:`, err.message))
