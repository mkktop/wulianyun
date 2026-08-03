# 网关子设备协议

> 网关设备（`isGateway=true`）可代理多个子设备接入：子设备不直接连接平台，而是通过网关在 MQTT `thing/gateway` 主题上登录/登出，平台接受子设备上报并经网关下行。
>
> 相关：鉴权 → [认证与安全](/guide/auth-security)

## 一、子设备登录 / 登出

### 主题

```
thing/gateway/{pk}/{gatewayName}/sub/{subId}/login
thing/gateway/{pk}/{gatewayName}/sub/{subId}/logout
```

- `{pk}` / `{gatewayName}`：网关设备自身的 ProductKey / DeviceName
- `{subId}`：子设备标识（与子设备 DeviceName 一致）

### 载荷

```json
{
  "productKey": "pk_sub",
  "deviceName": "sub-dev-1",
  "secret": "sub-secret-123",
  "timestamp": 1785712345678
}
```

| 字段 | 说明 |
|---|---|
| `productKey` / `deviceName` | 子设备的三元组标识 |
| `secret` | 子设备密钥（一机一密）或产品 `ProductSecret`（一型一密） |
| `timestamp` | 可选 |

### 处理逻辑

- **login**：校验网关设备 `IsGateway == true`；子设备经 `FindDeviceForAuth` 鉴权（兼容一型一密动态注册）后绑定 `gateway_id` 并置为 `online`。平台推送 `device_status` 事件（带 `via:"gateway"`、`gateway` 字段）
- **logout**：子设备置为 `offline`

## 二、子设备上行

子设备数据经网关设备本身的 `thing/up/{pk}/{gatewayName}` 主题上报（子设备无独立 MQTT 连接）。

## 三、子设备下行

平台向 `thing/gateway/{pk}/{gatewayName}/sub/{subDeviceName}` 下发（QoS 1）：

```
thing/gateway/{pk}/{gatewayName}/sub/{subDeviceName}
```

网关订阅该主题（ACL 允许 `thing/gateway/{pk}/{dn}/sub/+`）后转发给子设备。

## 四、管理 API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/devices/:id/sub-devices` | 添加子设备 |
| GET | `/api/v1/devices/:id/sub-devices` | 子设备列表 |
| DELETE | `/api/v1/devices/:id/sub-devices/:subId` | 移除子设备 |

## 五、ACL 权限

| 方向 | 主题 |
|---|---|
| 网关 publish | `thing/gateway/{pk}/{dn}/sub/{subId}/login` / `logout` |
| 网关 subscribe | `thing/gateway/{pk}/{dn}/sub/+`（**唯一允许 `+` 通配符的场景**） |