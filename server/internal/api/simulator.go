package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"

	"github.com/gin-gonic/gin"
)

type simSession struct {
	ID         string    `json:"id"`
	UserID     uint      `json:"userId"`
	ProductID  uint      `json:"productDbId"`
	DeviceID   uint      `json:"deviceId"`
	ProductKey string    `json:"productId"`
	DeviceName string    `json:"deviceName"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
}

var (
	simMu       sync.RWMutex
	simSessions = make(map[string]*simSession)
	// maxSimSessions 模拟会话上限；simSessionTTL 会话最大存活时长（防无限增长）
	maxSimSessions = 1000
	simSessionTTL  = 1 * time.Hour
)

// sweepSimSessions 清理过期会话；超过上限时按创建时间淘汰最旧（调用方需持 simMu 写锁）
func sweepSimSessions() {
	now := time.Now()
	for id, s := range simSessions {
		if s.Status == "disconnected" || now.Sub(s.CreatedAt) > simSessionTTL {
			delete(simSessions, id)
		}
	}
	if len(simSessions) >= maxSimSessions {
		// 淘汰最旧的 N 个（保新会话）
		type kv struct {
			id string
			t  time.Time
		}
		list := make([]kv, 0, len(simSessions))
		for id, s := range simSessions {
			list = append(list, kv{id, s.CreatedAt})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].t.Before(list[j].t) })
		for _, e := range list[:len(list)-maxSimSessions+1] {
			delete(simSessions, e.id)
		}
	}
}

func ConnectSimulator(c *gin.Context) {
	var req struct {
		ProductID uint `json:"productId" binding:"required"`
		DeviceID  uint `json:"deviceId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 1, err.Error())
		return
	}
	// 归属校验：产品必须可写定义（owner/超管），设备必须在可见范围内且属于该产品，
	// 防止跨租户冒认设备（IDOR）
	p, err := mustOwnProduct(c, req.ProductID)
	if err != nil {
		Fail(c, 1, "product not found")
		return
	}
	var d model.Device
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", req.DeviceID).First(&d).Error; err != nil {
		Fail(c, 1, "device not found")
		return
	}
	if d.ProductID != p.ID {
		Fail(c, 1, "设备不属于该产品")
		return
	}

	sessionID := generateSimID()
	sess := &simSession{
		ID: sessionID, UserID: UID(c), ProductID: p.ID, DeviceID: d.ID,
		ProductKey: p.ProductKey, DeviceName: d.Name, Status: "connected", CreatedAt: time.Now(),
	}
	simMu.Lock()
	sweepSimSessions()
	simSessions[sessionID] = sess
	simMu.Unlock()

	// clientID 须为 {productKey}{deviceName} 纯拼接（与 MQTT 真实 clientID 同格式），
	// 须经 ParseClientID 按字符数解析——勿加分隔点号
	clientID := p.ProductKey + d.Name
	service.QueueStatus(clientID, true, 0)
	OK(c, sess)
}

func PublishSimulator(c *gin.Context) {
	var req struct {
		SessionID string      `json:"sessionId" binding:"required"`
		Payload   interface{} `json:"payload" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 1, err.Error())
		return
	}
	simMu.RLock()
	sess, ok := simSessions[req.SessionID]
	simMu.RUnlock()
	if !ok || sess.Status != "connected" {
		Fail(c, 1, "session not found or disconnected")
		return
	}
	// 会话归属校验：只能向自己创建的模拟会话下发
	if sess.UserID != UID(c) {
		Fail(c, 1, "session not found")
		return
	}
	payload, _ := json.Marshal(req.Payload)
	go service.HandleTelemetry(sess.ProductKey, sess.DeviceName, payload)
	OK(c, gin.H{"status": "sent"})
}

func DisconnectSimulator(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 1, err.Error())
		return
	}
	simMu.Lock()
	sess, ok := simSessions[req.SessionID]
	if ok {
		sess.Status = "disconnected"
		delete(simSessions, req.SessionID)
	}
	simMu.Unlock()
	// 会话归属校验：只能断开自己创建的模拟会话
	if ok && sess.UserID != UID(c) {
		Fail(c, 1, "session not found")
		return
	}
	if ok {
		// clientID 须为纯拼接（见 StartSimulator 同名注释）
		clientID := sess.ProductKey + sess.DeviceName
		service.QueueStatus(clientID, false, 0)
	}
	OK(c, nil)
}

func ListSimulatorSessions(c *gin.Context) {
	simMu.RLock()
	list := make([]*simSession, 0, len(simSessions))
	for _, s := range simSessions {
		if s.UserID == UID(c) {
			list = append(list, s)
		}
	}
	simMu.RUnlock()
	OK(c, list)
}

func generateSimID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
