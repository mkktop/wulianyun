// DTU 模拟器：模拟 TCP 透传设备（对标有人云 DTU）
// 注册包: {productKey},{deviceName},{secret}\n → 平台回复 OK
// 之后每 5 秒上报 4 字节二进制帧: int16BE 温度x10 + int16BE 湿度x10
// 用法: node dtu.js <productKey> <deviceName> <deviceSecret> [host] [port]
const net = require('net')

const [, , productKey, deviceName, secret, host = '127.0.0.1', port = 9100] = process.argv
if (!productKey || !deviceName || !secret) {
  console.log('用法: node dtu.js <productKey> <deviceName> <deviceSecret> [host] [port]')
  process.exit(1)
}

let registered = false
let timer = null

function connect() {
  registered = false
  const sock = net.connect({ host, port: Number(port) }, () => {
    console.log(`[${deviceName}] TCP 已连接 ${host}:${port}，发送注册包`)
    sock.write(`${productKey},${deviceName},${secret}\n`)
  })

  sock.on('data', (data) => {
    const text = data.toString().trim()
    if (!registered) {
      if (text === 'OK') {
        registered = true
        console.log(`[${deviceName}] 注册成功，开始上报二进制帧`)
        timer = setInterval(() => {
          const temp = Math.round((20 + Math.random() * 10) * 10) // 温度x10
          const hum = Math.round((40 + Math.random() * 30) * 10)  // 湿度x10
          const frame = Buffer.alloc(4)
          frame.writeInt16BE(temp, 0)
          frame.writeInt16BE(hum, 2)
          sock.write(frame)
          console.log(`[${deviceName}] 上报帧: ${frame.toString('hex')} (temp=${temp / 10}, hum=${hum / 10})`)
        }, 5000)
      } else {
        console.error(`[${deviceName}] 注册失败: ${text}`)
        sock.destroy()
      }
      return
    }
    console.log(`[${deviceName}] 收到下行: hex=${data.toString('hex')} text=${text}`)
  })

  sock.on('close', () => {
    if (timer) clearInterval(timer)
    console.log(`[${deviceName}] 连接断开，3 秒后重连`)
    setTimeout(connect, 3000)
  })

  sock.on('error', (err) => console.error(`[${deviceName}] 错误:`, err.message))
}

connect()
