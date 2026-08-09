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

- `{pk}` / `{gatewayName}`：网关设备自身的 ProductID / DeviceName
- `{subId}`：子设备标识（与子设备 DeviceName 一致）

### 载荷

```json
{
  "productId": "pk_sub",
  "deviceName": "sub-dev-1",
  "secret": "sub-secret-123",
  "timestamp": 1785712345678
}
```

| 字段 | 说明 |
|---|---|
| `productId` / `deviceName` | 子设备的三元组标识 |
| `secret` | 子设备密钥（一机一密）或产品 `ProductSecret`（一型一密） |
| `timestamp` | 可选 |

### 处理逻辑

- **login**：要求网关设备已标记为网关类型；子设备经三元组校验（一机一密 / 一型一密，一型一密下可动态注册）后绑定到该网关并置为在线。平台推送设备状态事件（标注经网关接入）
- **logout**：子设备置为离线（绑定关系保留，再次 login 即恢复在线）

## 二、子设备上行

子设备数据经网关设备本身的 `thing/up/{pk}/{gatewayName}` 主题上报（子设备无独立 MQTT 连接）。

## 三、子设备下行

平台向网关下发针对某子设备的指令，主题使用**子设备名**（非 subId）：

```
thing/gateway/{pk}/{gatewayName}/sub/{subDeviceName}
```

网关订阅 `thing/gateway/{pk}/{gatewayName}/sub/+`（ACL 唯一允许 `+` 通配符的场景）后，按子设备名转发给对应子设备。

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