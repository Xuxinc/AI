package handlers

import (
	"ai-celebrity-simulator/services"
	"ai-celebrity-simulator/utils"

	"github.com/gin-gonic/gin"
)

// MaxAudioFileSize 文件大小限制常量
const (
	MaxAudioFileSize = 10 * 1024 * 1024 // 10MB
)

// VoiceModelFolder 文件夹名称常量
const (
	VoiceModelFolder = "voice_model"
)

// 响应消息常量
const (
	MessageSelectAudioFile    = "请选择音频文件"
	MessageAudioFileTooLarge  = "音频文件不能超过10MB"
	MessageUnsupportedFormat  = "不支持的音频格式，请上传WAV、MP3或M4A格式"
	MessageAudioUploadError   = "上传音频失败"
	MessageAudioUploadSuccess = "音频上传成功"
)

// AllowedAudioTypes 支持的音频文件类型
var AllowedAudioTypes = map[string]bool{
	"audio/wav":                true,
	"audio/mp3":                true,
	"audio/mp4":                true,
	"audio/m4a":                true,
	"audio/wave":               true,
	"audio/x-m4a":              true,
	"application/octet-stream": true, // 某些手机可能返回这个类型
}

// AllowedAudioExtensions 支持的音频文件扩展名
var AllowedAudioExtensions = map[string]bool{
	".mp3": true,
	".wav": true,
	".m4a": true,
}

// UploadVoiceAudio 上传音频文件
func UploadVoiceAudio(c *gin.Context) {
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageSelectAudioFile,
		})
		return
	}

	// 检查文件大小（10MB以内）
	if file.Size > MaxAudioFileSize {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageAudioFileTooLarge,
		})
		return
	}

	// 检查文件格式（兼容手机端MIME类型检测不准确的问题）
	contentType := file.Header.Get("Content-Type")
	if !AllowedAudioTypes[contentType] {
		// 如果MIME类型不支持，检查文件扩展名
		// 获取文件扩展名
		ext := ""
		if len(file.Filename) > 0 {
			// 查找最后一个点号
			for i := len(file.Filename) - 1; i >= 0; i-- {
				if file.Filename[i] == '.' {
					ext = file.Filename[i:]
					break
				}
			}
		}

		if !AllowedAudioExtensions[ext] {
			c.JSON(StatusBadRequest, gin.H{
				"code":    CodeBadRequest,
				"message": MessageUnsupportedFormat,
			})
			return
		}
	}

	// 上传到OSS的voice_model文件夹
	audioURL, err := services.UploadFileToOSS(file, VoiceModelFolder)
	if err != nil {
		utils.LogError("上传音频文件失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageAudioUploadError,
		})
		return
	}

	utils.LogInfo("音频文件上传成功 - 文件大小: %d bytes", file.Size)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageAudioUploadSuccess,
		"data": gin.H{
			"audio_url": audioURL,
		},
	})
}
