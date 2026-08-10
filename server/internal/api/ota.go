package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/mqtt"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"
	"iot-platform/internal/storage"

	"github.com/gin-gonic/gin"
)

// firmwareExtAllowed 固件上传扩展名白名单（防止上传 .html 等被同源静态渲染）
var firmwareExtAllowed = map[string]bool{
	".bin": true, ".hex": true, ".img": true, ".dat": true,
	".zip": true, ".tar": true, ".gz": true, ".pack": true, ".rbl": true,
}

// maxFirmwareUpload 固件上传 body 上限（与 nginx client_max_body_size 512m 对齐）
const maxFirmwareUpload = 512 << 20

func ListFirmwares(c *gin.Context) {
	q := repository.DB.Scopes(ownedScope(c, ""))
	if pid := c.Query("productId"); pid != "" {
		q = q.Where("product_id = ?", pid)
	}
	var list []model.Firmware
	q.Order("id desc").Find(&list)

	// 关联产品名称
	productNames := make(map[uint]string)
	for i := range list {
		if _, ok := productNames[list[i].ProductID]; !ok {
			var p model.Product
			repository.DB.Select("name").First(&p, list[i].ProductID)
			productNames[list[i].ProductID] = p.Name
		}
	}

	OK(c, gin.H{
		"list":         list,
		"productNames": productNames,
	})
}

// CreateFirmware 上传固件（multipart form-data）
func CreateFirmware(c *gin.Context) {
	// 从 form 字段读取（非 JSON）
	productIDStr := c.PostForm("productId")
	if productIDStr == "" {
		Fail(c, 400, "产品ID必填")
		return
	}
	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		Fail(c, 400, "产品ID格式错误")
		return
	}
	version := c.PostForm("version")
	if version == "" {
		Fail(c, 400, "版本号必填")
		return
	}
	description := c.PostForm("description")

	// 校验产品归属（owner 或被下放）
	if _, e := canViewProduct(c, productID); e != nil {
		Fail(c, 404, "产品不存在")
		return
	}

	// 处理文件上传（服务端强制 body 上限，nginx client_max_body_size 之外的第二道防线，防涨盘）
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFirmwareUpload)
	var fileURL string
	var fileSize int64
	var checksum string

	file, header, err := c.Request.FormFile("file")
	if err == nil {
		defer file.Close()
		// 安全文件名：只保留字母数字和点，防止路径穿越
		safeName := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
				return r
			}
			return '_'
		}, header.Filename)
		// 扩展名白名单：只允许固件二进制类型，杜绝 .html/.svg 等被同源渲染（存储型 XSS）
		ext := strings.ToLower(filepath.Ext(safeName))
		if !firmwareExtAllowed[ext] {
			Fail(c, 400, "仅支持固件文件类型（bin/hex/img/zip/tar/gz/pack 等）")
			return
		}
		// 对象名：firmware/{产品ID}_{版本}_{时间戳}_{随机串}_{文件名}
		// 随机串使 URL 不可猜测（公开读桶下代替签名防越权下载），且 URL 短而永久，适配 4G 模组
		randBytes := make([]byte, 4)
		if _, err := rand.Read(randBytes); err != nil {
			Fail(c, 500, "生成对象名失败")
			return
		}
		key := fmt.Sprintf("firmware/%d_%s_%d_%x_%s",
			productID, version, time.Now().Unix(), randBytes, safeName)

		// 边传边算 SHA-256 checksum（与本地模式一致，设备下载后据此校验完整性）
		hasher := sha256.New()
		if err := storage.Default.Put(c.Request.Context(), key, io.TeeReader(file, hasher), header.Size); err != nil {
			slog.Error("store firmware failed", "key", key, "err", err)
			Fail(c, 500, "保存文件失败")
			return
		}
		fileURL = storage.Default.URL(key)
		fileSize = header.Size
		checksum = hex.EncodeToString(hasher.Sum(nil))
	}

	fw := model.Firmware{
		UserID:      UID(c),
		ProductID:   uint(productID),
		Version:     version,
		FileURL:     fileURL,
		FileSize:    fileSize,
		Checksum:    checksum,
		Description: description,
	}
	if err := repository.DB.Create(&fw).Error; err != nil {
		// 记录入库失败时回收已存储的对象，避免产生孤儿文件
		if key := storage.Default.KeyFromURL(fileURL); key != "" {
			if derr := storage.Default.Delete(c.Request.Context(), key); derr != nil {
				slog.Warn("cleanup firmware object failed", "key", key, "err", derr)
			}
		}
		Fail(c, 500, "保存固件记录失败")
		return
	}
	OK(c, fw)
}

// DeleteFirmware 删除固件（同时删除存储对象）
func DeleteFirmware(c *gin.Context) {
	var fw model.Firmware
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&fw).Error; err != nil {
		Fail(c, 404, "固件不存在")
		return
	}
	// 删除存储对象（本地磁盘 / 对象存储统一走 storage 抽象；URL 非本存储返回空 key 则跳过）
	if fw.FileURL != "" {
		if key := storage.Default.KeyFromURL(fw.FileURL); key != "" {
			if err := storage.Default.Delete(c.Request.Context(), key); err != nil {
				slog.Warn("delete firmware file failed", "key", key, "err", err)
			}
		}
	}
	repository.DB.Delete(&fw)
	OK(c, nil)
}

// ---- OTA 升级任务 ----

// CreateOTATask 创建升级任务并立即下发 MQTT 升级通知
func CreateOTATask(c *gin.Context) {
	var req struct {
		FirmwareID uint   `json:"firmwareId" binding:"required"`
		DeviceIDs  []uint `json:"deviceIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, 400, err.Error())
		return
	}
	var fw model.Firmware
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", req.FirmwareID).First(&fw).Error; err != nil {
		Fail(c, 404, "固件不存在")
		return
	}

	// 校验设备归属并查询设备信息（且必须与固件属于同一产品，防止把 A 产品固件推给 B 设备）
	var devices []model.Device
	repository.DB.Scopes(ownedScope(c, "")).Where("id IN ? AND product_id = ?", req.DeviceIDs, fw.ProductID).Find(&devices)
	if len(devices) == 0 {
		Fail(c, 400, "未找到有效设备（需与固件同属一个产品）")
		return
	}
	// 剔除不属于该产品的设备 ID，避免下发时越产品推送
	validIDs := make([]uint, 0, len(devices))
	validSet := make(map[uint]bool, len(devices))
	for _, d := range devices {
		validSet[d.ID] = true
	}
	for _, id := range req.DeviceIDs {
		if validSet[id] {
			validIDs = append(validIDs, id)
		}
	}

	idsJSON, _ := json.Marshal(validIDs)
	task := model.OTATask{
		UserID: UID(c), FirmwareID: fw.ID, ProductID: fw.ProductID,
		DeviceIDs: string(idsJSON), Status: "running",
	}
	repository.DB.Create(&task)

	// 下发 OTA 升级通知
	otaPayload, _ := json.Marshal(map[string]interface{}{
		"method":  "ota.push",
		"version": fw.Version,
		"url":     fw.FileURL,
		"size":    fw.FileSize,
		"sha256":  fw.Checksum,
		"taskId":  task.ID,
		"ts":      time.Now().UnixMilli(),
	})

	successCount := 0
	for _, d := range devices {
		// 优先走统一下行通道（支持 TCP 在线设备），失败则直接 MQTT
		if service.DownPublisher != nil {
			if err := service.DownPublisher(d.ProductKey, d.Name, otaPayload); err == nil {
				successCount++
				continue
			}
		}
		// fallback: 直接 MQTT
		if err := mqtt.PublishDown(d.ProductKey, d.Name, otaPayload); err == nil {
			successCount++
		}
	}

	slog.Info("ota task created and pushed", "taskId", task.ID, "firmware", fw.Version,
		"devices", len(devices), "pushed", successCount)

	OK(c, gin.H{
		"task":         task,
		"pushedCount":  successCount,
		"totalDevices": len(devices),
	})
}

// ListOTATasks 列出升级任务（关联固件版本信息）
func ListOTATasks(c *gin.Context) {
	var tasks []model.OTATask
	repository.DB.Scopes(ownedScope(c, "")).Order("id desc").Find(&tasks)

	// 关联固件信息
	firmwareMap := make(map[uint]model.Firmware)
	for i := range tasks {
		fid := tasks[i].FirmwareID
		if _, ok := firmwareMap[fid]; !ok {
			var fw model.Firmware
			repository.DB.First(&fw, fid)
			firmwareMap[fid] = fw
		}
	}

	// 构建结果
	type taskWithInfo struct {
		model.OTATask
		FirmwareVersion string `json:"firmwareVersion"`
		ProductName     string `json:"productName"`
		DeviceCount     int    `json:"deviceCount"`
	}
	var result []taskWithInfo
	productNames := make(map[uint]string)
	for i := range tasks {
		t := tasks[i]
		fw := firmwareMap[t.FirmwareID]
		deviceCount := 0
		var ids []int
		json.Unmarshal([]byte(t.DeviceIDs), &ids)
		deviceCount = len(ids)

		pName := ""
		if pn, ok := productNames[t.ProductID]; ok {
			pName = pn
		} else {
			var p model.Product
			repository.DB.Select("name").First(&p, t.ProductID)
			productNames[t.ProductID] = p.Name
			pName = p.Name
		}

		result = append(result, taskWithInfo{
			OTATask:         t,
			FirmwareVersion: fw.Version,
			ProductName:     pName,
			DeviceCount:     deviceCount,
		})
	}

	OK(c, result)
}

// HandleOTAProgress 已移至 service 包，此处保留兼容性注释
// 请使用 service.HandleOTAProgress(deviceID, payload)
