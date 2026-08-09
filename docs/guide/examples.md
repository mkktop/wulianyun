# 示例代码

> 一站式最小可运行示例。所有示例假定已取得设备三元组（`ProductID / DeviceName / DeviceSecret`），平台地址为 `http://<平台地址>`。

## 一、MQTT 上报 + 接收下行（Node.js）

```js
const mqtt = require('mqtt')

const productId = 'pk...'
const deviceName = 'dev1'
const secret = '...'

const client = mqtt.connect('mqtt://<平台地址>:1883', {
  // ClientID = 产品ID + 设备名 纯拼接（无分隔符），如 'AB1234567890dev1'
  clientId: `${productId}${deviceName}`,
  username: deviceName,
  password: secret,
  reconnectPeriod: 3000,
})

const up = `thing/up/${productId}/${deviceName}`
const down = `thing/down/${productId}/${deviceName}`

client.on('connect', () => {
  console.log('已连接')
  client.subscribe(down, { qos: 1 })
  // 每 5 秒上报一次遥测
  setInterval(() => {
    client.publish(up, JSON.stringify({
      temperature: +(20 + Math.random() * 10).toFixed(2),
      humidity: +(40 + Math.random() * 30).toFixed(2),
    }), { qos: 1 })
  }, 5000)
})

client.on('message', (topic, payload) => {
  const msg = JSON.parse(payload.toString())
  if (msg.method === 'property.set') {
    // 应用属性并上报新值（平台据此清除影子 desired）
    client.publish(up, JSON.stringify(msg.params), { qos: 1 })
  }
  if (msg.method === 'service.invoke') {
    console.log('执行服务', msg.service, msg.params)
  }
})
```

## 二、事件上报

```js
client.publish(up, JSON.stringify({
  method: 'event.post',
  identifier: 'highTemp',
  type: 'alert',
  params: { temperature: 35.2 },
}), { qos: 1 })
```

## 三、TCP 透传（DTU）注册 + 上报

```js
const net = require('net')

const sock = net.connect({ host: '<平台地址>', port: 9100 })

sock.on('data', (chunk) => {
  const text = chunk.toString()
  if (text.includes('OK')) {
    console.log('注册成功')
    // 上报 4 字节二进制帧（配合 decode 脚本）
    const frame = Buffer.alloc(4)
    frame.writeInt16BE(250, 0)  // 温度 x10 = 25.0
    frame.writeInt16BE(550, 2)  // 湿度 x10 = 55.0
    sock.write(frame)
  }
})

// 注册包：三元组逗号分隔，\n 结尾
sock.write(`${productId},${deviceName},${secret}\n`)
```

## 四、HTTP 直传

```bash
curl -X POST http://<平台地址>/api/v1/http/telemetry \
  -H "X-Device-Token: $(printf 'pk123:dev1:secret123' | base64)" \
  -H "Content-Type: application/json" \
  -d '{"temperature":25.5,"humidity":60.2}'
```

## 五、OpenAPI 签名访问

### Node.js

```js
const crypto = require('crypto')

const appKey = 'ak3f8a1c...'
const appSecret = 'c9d2f1e8...'
const ts = Math.floor(Date.now() / 1000)
const method = 'GET'
const path = '/openapi/v1/devices/3/latest'
const bodyHash = crypto.createHash('sha256').update('').digest('hex')
// 待签名串 = Method\nPathAndQuery\nBodyHash\nAppKey\nTimestamp
const sign = crypto.createHmac('sha256', appSecret)
  .update(`${method}\n${path}\n${bodyHash}\n${appKey}\n${ts}`).digest('hex')

const res = await fetch(`http://<平台地址>${path}`, {
  headers: {
    'X-App-Key': appKey,
    'X-Timestamp': String(ts),
    'X-Sign': sign,
  },
})
console.log(await res.json())
```

### Node.js（POST 带请求体）

> 签名中的 `bodyHash` 必须与**实际发送的请求体字节**完全一致（任何空格/换行差异都会导致 401 签名错误）。

```js
const body = JSON.stringify({ params: { switch: 1 }, expireSec: 0 })
const path = '/openapi/v1/devices/3/property'
const bodyHash = crypto.createHash('sha256').update(body).digest('hex')
const sign = crypto.createHmac('sha256', appSecret)
  .update(`POST\n${path}\n${bodyHash}\n${appKey}\n${ts}`).digest('hex')

const res = await fetch(`http://<平台地址>${path}`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-App-Key': appKey,
    'X-Timestamp': String(ts),
    'X-Sign': sign,
  },
  body,
})
```

### Python

```python
import time, hmac, hashlib, requests

app_key = "ak3f8a1c..."
app_secret = "c9d2f1e8..."
ts = str(int(time.time()))
path = "/openapi/v1/devices/3/latest"
body_hash = hashlib.sha256(b"").hexdigest()
raw = f"GET\n{path}\n{body_hash}\n{app_key}\n{ts}"
sign = hmac.new(app_secret.encode(), raw.encode(), hashlib.sha256).hexdigest()

r = requests.get(
    "http://<平台地址>" + path,
    headers={"X-App-Key": app_key, "X-Timestamp": ts, "X-Sign": sign},
)
print(r.json())
```

### Python（POST 带请求体）

```python
body = b'{"params": {"switch": 1}, "expireSec": 0}'
path = "/openapi/v1/devices/3/property"
body_hash = hashlib.sha256(body).hexdigest()
raw = f"POST\n{path}\n{body_hash}\n{app_key}\n{ts}"
sign = hmac.new(app_secret.encode(), raw.encode(), hashlib.sha256).hexdigest()

r = requests.post(
    "http://<平台地址>" + path,
    data=body,
    headers={"Content-Type": "application/json",
             "X-App-Key": app_key, "X-Timestamp": ts, "X-Sign": sign},
)
print(r.json())
```

## 六、换取动态令牌（tk:）

```bash
curl -X POST http://<平台地址>/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"productId":"pk123","deviceName":"dev1","secret":"secret123"}'
```

```json
{ "code": 0, "data": { "token": "tk:1f2e...", "ttl": 3600 } }
```

之后用 `tk:1f2e...` 作为 MQTT password 连接即可。

## 七、内置模拟器

```bash
cd tools/simulator
npm install

# MQTT 设备：每 5 秒上报温湿度，接收下行命令与影子
node simulator.js <productId> <deviceName> <deviceSecret> [broker]

# TCP 透传（DTU）设备
node dtu.js <productId> <deviceName> <deviceSecret> [host] [port]

# TCP + Modbus 从机模拟
node dtu-modbus.js <productId> <deviceName> <deviceSecret> [host] [port]
```

## 八、WebSocket 实时订阅（管理端）

> 认证方式为**首帧认证**：升级后 5 秒内必须发送 `{"type":"auth","token":"<JWT>"}`，token 不放进 URL（防泄露进访问日志）；认证失败服务端返回 `{"type":"auth_failed"}` 并关闭连接。

```js
const ws = new WebSocket(`ws://<平台地址>/api/v1/ws`)

ws.onopen = () => {
  // 首帧认证（必须，5 秒内）
  ws.send(JSON.stringify({ type: 'auth', token: '<JWT>' }))
  // 认证后订阅设备遥测（deviceId 为数据库主键）
  ws.send(JSON.stringify({ type: 'subscribe', deviceId: 3 }))
}

ws.onmessage = (e) => {
  const msg = JSON.parse(e.data)
  if (msg.type === 'auth_failed') {
    // token 失效：跳登录
    location.href = '/login'
    return
  }
  // { type: 'telemetry', deviceId: 3, payload: { ts, data } }
  // { type: 'device_status', deviceId: 3, payload: { status, ts } }
  // { type: 'alarm', payload: { ruleName, level, message } }
  console.log(msg)
}
```