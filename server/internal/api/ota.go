package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"iot-platform/internal/model"
	"iot-platform/internal/mqtt"
	"iot-platform/internal/repository"
	"iot-platform/internal/service"

	"github.com/gin-gonic/gin"
)

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

	// 处理文件上传
	var fileURL string
	var fileSize int64
	var checksum string

	file, header, err := c.Request.FormFile("file")
	if err == nil {
		defer file.Close()
		dir := filepath.Join("uploads", "firmware")
		os.MkdirAll(dir, 0755)
		// 安全文件名：只保留字母数字和点，防止路径穿越
		safeName := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
				return r
			}
			return '_'
		}, header.Filename)
		filename := fmt.Sprintf("%d_%s_%d_%s", productID, version, time.Now().Unix(), safeName)
		// 确保 filepath.Join 后仍在 dir 下
		dst := filepath.Join(dir, filename)
		if !strings.HasPrefix(filepath.Clean(dst), filepath.Clean(dir)+string(filepath.Separator)) {
			Fail(c, 400, "非法文件名")
			return
		}
		out, err := os.Create(dst)
		if err != nil {
			Fail(c, 500, "保存文件失败")
			return
		}

		// 同时计算 SHA-256 checksum
		hasher := sha256.New()
		_, err = io.Copy(io.MultiWriter(out, hasher), file)
		out.Close()
		if err != nil {
			Fail(c, 500, "写入文件失败")
			return
		}
		fileURL = "/" + filepath.ToSlash(dst)
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
	repository.DB.Create(&fw)
	OK(c, fw)
}

// DeleteFirmware 删除固件（同时删除物理文件）
func DeleteFirmware(c *gin.Context) {
	var fw model.Firmware
	if err := repository.DB.Scopes(ownedScope(c, "")).Where("id = ?", c.Param("id")).First(&fw).Error; err != nil {
		Fail(c, 404, "固件不存在")
		return
	}
	// 删除物理文件（安全拼接，防止路径穿越）
	if fw.FileURL != "" {
		path := "." + fw.FileURL
		cleanPath := filepath.Clean(path)
		baseDir := filepath.Clean(filepath.Join(".", "uploads", "firmware"))
		if strings.HasPrefix(cleanPath, baseDir+string(filepath.Separator)) {
			if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("delete firmware file failed", "path", cleanPath, "err", err)
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

	// 校验设备归属并查询设备信息
	var devices []model.Device
	repository.DB.Scopes(ownedScope(c, "")).Where("id IN ?", req.DeviceIDs).Find(&devices)
	if len(devices) == 0 {
		Fail(c, 400, "未找到有效设备")
		return
	}

	idsJSON, _ := json.Marshal(req.DeviceIDs)
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
