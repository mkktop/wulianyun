// Modbus DTU 模拟器：模拟 4G DTU（三元组注册）+ 串口 Modbus RTU 从机
// 开机主动连平台 9100 → 三元组注册 → 平台按周期下发 RTU 读请求，本模拟器应答
// 内置保持寄存器：地址0=温度x10(~25.x), 地址1=湿度x10(~55.x), 地址16=开关(可写)
// 用法: node dtu-modbus.js <productKey> <deviceName> <secret> [host] [port]
const net = require('net')

const [, , productKey, deviceName, secret, host = '127.0.0.1', port = 9100] = process.argv
if (!productKey || !deviceName || !secret) {
  console.log('用法: node dtu-modbus.js <productKey> <deviceName> <deviceSecret> [host] [port]')
  process.exit(1)
}

// 模拟保持寄存器（16 位无符号），可被平台读写
const registers = {
  0: 250,   // 温度 x10 = 25.0（连续地址 0/1/2 会被平台合并成一次请求）
  1: 550,   // 湿度 x10 = 55.0
  2: 1013,  // 气压
  10: 1,    // 状态位（稳定，用于演示变更上报抑制）
  16: 0     // 开关 0/1（可写）
}

function crc16(buf) {
  let crc = 0xFFFF
  for (let i = 0; i < buf.length; i++) {
    crc ^= buf[i]
    for (let j = 0; j < 8; j++) {
      if (crc & 1) crc = (crc >> 1) ^ 0xA001
      else crc >>= 1
    }
  }
  return crc
}

function appendCRC(bytes) {
  const crc = crc16(Buffer.from(bytes))
  return Buffer.from([...bytes, crc & 0xFF, (crc >> 8) & 0xFF])
}

// 处理平台下发的 Modbus RTU 请求帧，返回应答帧
function handleRequest(frame) {
  if (frame.length < 4) return null
  const slave = frame[0]
  const func = frame[1]
  const addr = (frame[2] << 8) | frame[3]

  // 读保持寄存器/输入寄存器 0x03/0x04（支持批量连续寄存器）
  if (func === 0x03 || func === 0x04) {
    const qty = (frame[4] << 8) | frame[5]
    const bytes = [slave, func, qty * 2]
    for (let i = 0; i < qty; i++) {
      const v = registers[addr + i] || 0
      bytes.push((v >> 8) & 0xFF, v & 0xFF)
    }
    // 温湿度气压波动；状态位(10)与开关(16)保持稳定
    registers[0] = 240 + Math.floor(Math.random() * 60)
    registers[1] = 400 + Math.floor(Math.random() * 300)
    registers[2] = 1000 + Math.floor(Math.random() * 30)
    return appendCRC(bytes)
  }
  // 写单个寄存器 0x06
  if (func === 0x06) {
    const val = (frame[4] << 8) | frame[5]
    registers[addr] = val
    console.log(`[${deviceName}] 写寄存器 addr=${addr} val=${val}`)
    return Buffer.from(frame) // 回显原帧
  }
  // 写单个线圈 0x05
  if (func === 0x05) {
    registers[addr] = frame[4] === 0xFF ? 1 : 0
    console.log(`[${deviceName}] 写线圈 addr=${addr} val=${registers[addr]}`)
    return Buffer.from(frame)
  }
  return null
}

let registered = false

function connect() {
  registered = false
  const sock = net.connect({ host, port: Number(port) }, () => {
    console.log(`[${deviceName}] TCP 已连接 ${host}:${port}，发送三元组注册包`)
    sock.write(`${productKey},${deviceName},${secret}\n`)
  })

  sock.on('data', (data) => {
    if (!registered) {
      const text = data.toString().trim()
      if (text === 'OK') {
        registered = true
        console.log(`[${deviceName}] 注册成功，等待平台轮询 Modbus 点位`)
      } else {
        console.error(`[${deviceName}] 注册失败: ${text}`)
        sock.destroy()
      }
      return
    }
    // 已注册：收到的都是 Modbus RTU 请求帧
    const resp = handleRequest(data)
    if (resp) {
      sock.write(resp)
      console.log(`[${deviceName}] 请求 ${data.toString('hex')} -> 应答 ${resp.toString('hex')}`)
    } else {
      console.log(`[${deviceName}] 未识别请求: ${data.toString('hex')}`)
    }
  })

  sock.on('close', () => {
    console.log(`[${deviceName}] 连接断开，3 秒后重连`)
    setTimeout(connect, 3000)
  })
  sock.on('error', (err) => console.error(`[${deviceName}] 错误:`, err.message))
}

connect()
