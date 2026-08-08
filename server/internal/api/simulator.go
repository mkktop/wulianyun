package api

import (
	"encoding/json"
	"fmt"
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
	ProductID  uint      `json:"productId"`
	DeviceID   uint      `json:"deviceId"`
	ProductKey string    `json:"productKey"`
	DeviceName string    `json:"deviceName"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
}

var (
	simMu       sync.RWMutex
	simSessions = make(map[string]*simSession)
)

func ConnectSimulator(c *gin.Context) {
	var req struct {
		ProductID uint `json:"productId" binding:"required"`
		DeviceID  uint `json:"deviceId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 1, err.Error())
		return
	}
	var d model.Device
	if err := repository.DB.First(&d, req.DeviceID).Error; err != nil {
		Fail(c, 1, "device not found")
		return
	}
	var p model.Product
	if err := repository.DB.First(&p, req.ProductID).Error; err != nil {
		Fail(c, 1, "product not found")
		return
	}

	sessionID := generateSimID()
	sess := &simSession{
		ID: sessionID, UserID: UID(c), ProductID: p.ID, DeviceID: d.ID,
		ProductKey: p.ProductKey, DeviceName: d.Name, Status: "connected", CreatedAt: time.Now(),
	}
	simMu.Lock()
	simSessions[sessionID] = sess
	simMu.Unlock()

	clientID := fmt.Sprintf("%s.%s", p.ProductKey, d.Name)
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
	if ok {
		clientID := fmt.Sprintf("%s.%s", sess.ProductKey, sess.DeviceName)
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
