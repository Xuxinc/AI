package handlers

import (
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/services"
	"ai-celebrity-simulator/utils"

	"github.com/gin-gonic/gin"
)

// 响应消息常量
const (
	MessageVoiceCreateSuccess = "音色创建成功"

	MessageRequestParamError = "请求参数错误"
	MessageAudioURLRequired  = "音频URL不能为空"
	MessagePrefixRequired    = "音色前缀不能为空"
	MessagePrefixTooLong     = "音色前缀不能超过10个字符"
	MessagePrefixInvalidChar = "音色前缀只能包含小写字母"
	MessageCreateVoiceError  = "创建音色失败"
	MessageUpdateDBError     = "更新数据库失败"
)

// MaxPrefixLength 音色前缀限制常量
const (
	MaxPrefixLength = 10
)

// CreateVoiceModel 创建音色模型
func CreateVoiceModel(c *gin.Context) {
	var req struct {
		AudioURL    string `json:"audio_url"`
		Prefix      string `json:"prefix"`
		CharacterID uint   `json:"character_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageRequestParamError,
		})
		return
	}

	// 验证必填字段
	if req.AudioURL == "" {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageAudioURLRequired,
		})
		return
	}

	if req.Prefix == "" {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessagePrefixRequired,
		})
		return
	}

	// 验证前缀格式（仅允许数字和小写字母，小于十个字符）
	if len(req.Prefix) > MaxPrefixLength {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessagePrefixTooLong,
		})
		return
	}

	// 检查前缀是否只包含小写字母（celebrity符合要求）
	for _, char := range req.Prefix {
		if !(char >= 'a' && char <= 'z') {
			c.JSON(StatusBadRequest, gin.H{
				"code":    CodeBadRequest,
				"message": MessagePrefixInvalidChar,
			})
			return
		}
	}

	// 调用阿里云音色复刻API
	voiceID, err := services.CreateVoiceModel(req.AudioURL, req.Prefix)
	if err != nil {
		utils.LogError("创建音色失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageCreateVoiceError,
		})
		return
	}

	// 更新数据库中的角色音色模型
	if err := database.DB.Model(&models.Character{}).Where("id = ?", req.CharacterID).Update("voice_model", voiceID).Error; err != nil {
		utils.LogError("更新数据库失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageUpdateDBError,
		})
		return
	}

	// 清除相关缓存，确保所有客户端都能看到最新的音色信息
	if err := utils.ClearCharacterCaches(req.CharacterID); err != nil {
		utils.LogWarn("⚠️ 清除角色缓存失败 - 角色ID: %d, 错误: %v", req.CharacterID, err)
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageVoiceCreateSuccess,
		"data": gin.H{
			"voice_id": voiceID,
		},
	})
}
