package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ipRateLimit 按源 IP 的固定窗口限流中间件（进程内）。
// 用途：登录/换token/注册的暴破防护、EMQX 回调与设备 HTTP 上报的刷量防护。
// 规则：
//   - 内网/回环来源不限流——EMQX 认证回调来自内网，万级设备重连风暴时
//     限流会误伤正常设备接入；公网攻击者经 nginx 反代到达时 ClientIP 已是真实公网 IP；
//   - 多实例部署时各实例独立计数——对单源爆破仍有效（爆破流量会命中其中某一实例）；
//   - 已配合 SetTrustedProxies 收紧，ClientIP 不可被 X-Forwarded-For 伪造绕过
func ipRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	counts := map[string]int{}
	windowStart := time.Now()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if parsed := net.ParseIP(ip); parsed != nil && (parsed.IsPrivate() || parsed.IsLoopback()) {
			c.Next() // 内网来源（EMQX 回调/nginx 本机等）不限
			return
		}
		mu.Lock()
		if time.Since(windowStart) >= window {
			counts = map[string]int{}
			windowStart = time.Now()
		}
		counts[ip]++
		over := counts[ip] > limit
		mu.Unlock()
		if over {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, Resp{Code: 429, Msg: "请求过于频繁，请稍后再试"})
			return
		}
		c.Next()
	}
}
