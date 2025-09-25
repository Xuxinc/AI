package handlers

import (
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/services"
	"ai-celebrity-simulator/utils"
	"fmt"
	"strconv"
	"strings"
	"time"

	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 响应消息常量
const (
	MessageSendSuccess = "发送成功"
	MessageSaveSuccess = "保存成功"

	MessageContentEmpty            = "消息内容不能为空"
	MessageDialogNotFound          = "会话不存在"
	MessageMessageNotFound         = "消息不存在或已被删除"
	MessageCreateDialogError       = "创建会话失败"
	MessageGetDialogError          = "获取对话失败"
	MessageGetConversationError    = "获取会话列表失败"
	MessageDeleteMessageError      = "删除消息失败"
	MessageDeleteDialogError       = "删除会话失败"
	MessagePinError                = "置顶失败"
	MessageImageServiceUnavailable = "图片上传服务不可用"
	MessageSelectImageFiles        = "请选择要上传的图片"
	MessageGetUploadFilesError     = "获取上传文件失败"
	MessageTooManyImages           = "最多只能上传5张图片"
	MessageMissingCharacterID      = "缺少角色ID参数"
	MessageSaveCallDurationError   = "保存通话时长失败"
	MessageFindDialogError         = "查找会话失败"
)

// 数据库字段值常量
const (
	RoleUser     = "user"
	RoleAI       = "ai"
	IsVoiceNo    = "no"
	IsTopNo      = "no"
	IsTopYes     = "yes"
	IsDeletedNo  = "no"
	IsDeletedYes = "yes"
)

// DefaultAIReply 默认回复常量
const (
	DefaultAIReply = "现在聊天还不可用！"
)

// MaxImageCount 限制常量
const (
	MaxImageCount = 5
)

// 数据库错误检查常量
const (
	DuplicateEntryError   = "Duplicate entry"
	UniqueConstraintError = "unique_user_character"
)

// clearConversationCachesSafely 安全地清除会话缓存，处理错误但不中断流程
func clearConversationCachesSafely(userID uint) {
	if err := utils.ClearConversationCaches(userID); err != nil {
		utils.LogWarn("⚠️ 清除会话缓存失败 - 用户ID: %d, 错误: %v", userID, err)
	}
}

// createDialogSafely 安全地创建会话，处理重复条目错误
func createDialogSafely(tx *gorm.DB, userID, characterID uint) (*models.Dialog, error) {
	dialog := models.Dialog{
		UserID:      userID,
		CharacterID: characterID,
		IsTop:       IsTopNo,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := tx.Create(&dialog).Error; err != nil {
		// 检查是否是唯一约束冲突
		if isDuplicateEntryError(err) {
			return handleDuplicateEntry(tx, userID, characterID)
		}
		utils.LogError("创建会话失败: %v", err)
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}

	utils.LogInfo("会话创建成功 - 会话ID: %d", dialog.ID)
	return &dialog, nil
}

// createDialogWithoutTransaction 在非事务环境中安全地创建会话
func createDialogWithoutTransaction(userID, characterID uint) (*models.Dialog, error) {
	dialog := models.Dialog{
		UserID:      userID,
		CharacterID: characterID,
		IsTop:       IsTopNo,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := database.DB.Create(&dialog).Error; err != nil {
		utils.LogError("创建会话失败: %v", err)
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}

	utils.LogInfo("会话创建成功 - 会话ID: %d", dialog.ID)
	return &dialog, nil
}

type ChatMessageRequest struct {
	CharacterID uint     `json:"character_id" binding:"required"`
	Message     string   `json:"message"`
	ImageUrls   []string `json:"image_urls"` // 新增：图片URL列表
}

type ChatMessageResponse struct {
	Reply    string `json:"reply"`
	AudioURL string `json:"audio_url"`
}

// validateChatRequest 验证聊天请求
func validateChatRequest(req ChatMessageRequest) error {
	if req.Message == "" && len(req.ImageUrls) == 0 {
		return errors.New(MessageContentEmpty)
	}
	return nil
}

// getCharacter 获取角色信息
func getCharacter(characterID uint) (*models.Character, error) {
	var character models.Character
	if err := database.DB.First(&character, characterID).Error; err != nil {
		utils.LogError("获取角色失败 - 角色ID: %d, 错误: %v", characterID, err)
		return nil, fmt.Errorf("角色不存在: %v", err)
	}
	return &character, nil
}

// findOrCreateDialog 查找或创建会话
func findOrCreateDialog(userID, characterID uint) (*models.Dialog, error) {
	// 先尝试查找现有会话
	var dialog models.Dialog
	if err := database.DB.Where("user_id = ? AND character_id = ?", userID, characterID).First(&dialog).Error; err == nil {
		return &dialog, nil
	}

	// 会话不存在，创建新会话
	return createDialogWithTransaction(userID, characterID)
}

// createDialogWithTransaction 使用事务创建会话
func createDialogWithTransaction(userID, characterID uint) (*models.Dialog, error) {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 在事务中再次检查会话是否存在
	var existingDialog models.Dialog
	if err := tx.Where("user_id = ? AND character_id = ?", userID, characterID).First(&existingDialog).Error; err == nil {
		// 会话已存在，使用现有会话
		utils.LogInfo("在事务中发现现有会话 - 会话ID: %d", existingDialog.ID)
		return &existingDialog, nil
	}

	// 创建新会话
	dialog, err := createDialogSafely(tx, userID, characterID)
	if err != nil {
		return nil, err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		utils.LogError("提交事务失败: %v", err)
		return nil, fmt.Errorf("提交事务失败: %v", err)
	}

	return dialog, nil
}

// isDuplicateEntryError 检查是否为重复条目错误
func isDuplicateEntryError(err error) bool {
	return strings.Contains(err.Error(), DuplicateEntryError) ||
		strings.Contains(err.Error(), UniqueConstraintError)
}

// handleDuplicateEntry 处理重复条目错误
func handleDuplicateEntry(tx *gorm.DB, userID, characterID uint) (*models.Dialog, error) {
	utils.LogInfo("检测到唯一约束冲突，尝试获取现有会话")

	var dialog models.Dialog
	if err := tx.Where("user_id = ? AND character_id = ?", userID, characterID).First(&dialog).Error; err != nil {
		utils.LogError("获取现有会话失败: %v", err)
		return nil, fmt.Errorf("获取现有会话失败: %v", err)
	}

	utils.LogInfo("成功获取现有会话 - 会话ID: %d", dialog.ID)
	return &dialog, nil
}

// createUserMessage 创建用户消息
func createUserMessage(dialogID uint, content string, imageUrls []string) *models.Message {
	pictureURL := ""
	if len(imageUrls) > 0 {
		pictureURL = strings.Join(imageUrls, ",")
	}

	userMessage := models.Message{
		DialogID:   dialogID,
		Content:    content,
		PictureURL: pictureURL,
		Role:       RoleUser,
		Time:       time.Now(),
	}

	database.DB.Create(&userMessage)
	return &userMessage
}

// generateAIReply 生成AI回复
func generateAIReply(dialogID uint, characterPrompt, userMessage string, imageUrls []string) string {

	utils.LogInfo("开始生成AI回复 - 会话ID: %d, 图片数量: %d", dialogID, len(imageUrls))

	var aiReply string
	var err error

	// 根据是否有图片选择不同的AI服务
	if len(imageUrls) > 0 {
		// 有图片，使用视觉模型
		aiReply, err = services.CallAIWithImages(dialogID, characterPrompt, userMessage, imageUrls)
	} else {
		// 纯文本，使用原有的多轮对话服务
		aiReply, err = services.GenerateAIReplyWithHistory(dialogID, characterPrompt, userMessage)
	}

	if err != nil {
		utils.LogError("AI服务调用失败: %v", err)
		// 如果AI服务失败，返回友好的提示信息
		aiReply = DefaultAIReply
	} else {
		utils.LogInfo("AI回复生成成功 - 会话ID: %d, 回复长度: %d", dialogID, len(aiReply))
	}

	return aiReply
}

// createAIMessage 创建AI消息
func createAIMessage(dialogID uint, content string) *models.Message {
	aiMessage := models.Message{
		DialogID:   dialogID,
		Content:    content,
		PictureURL: "", // AI回复通常没有图片，但保持字段一致性
		Role:       RoleAI,
		Time:       time.Now(),
	}

	database.DB.Create(&aiMessage)
	return &aiMessage
}

// updateDialogTime 更新会话时间
func updateDialogTime(dialog *models.Dialog) {
	dialog.UpdatedAt = time.Now()
	database.DB.Save(dialog)
}

// validateCharacterID 验证角色ID参数
func validateCharacterID(characterIDStr string) (uint, error) {
	if characterIDStr == "" {
		return 0, errors.New(MessageMissingCharacterID)
	}

	characterID, err := strconv.ParseUint(characterIDStr, 10, 32)
	if err != nil {
		return 0, errors.New(MessageInvalidCharacterID)
	}

	return uint(characterID), nil
}

// createWelcomeMessage 创建欢迎消息
func createWelcomeMessage(dialogID uint, characterName string) error {
	return createWelcomeMessageWithDB(database.DB, dialogID, characterName)
}

// createWelcomeMessageWithDB 使用指定的数据库连接创建欢迎消息
func createWelcomeMessageWithDB(db *gorm.DB, dialogID uint, characterName string) error {
	welcomeMessage := models.Message{
		DialogID:   dialogID,
		Content:    "你好，我是" + characterName + "，很高兴和你聊天！",
		PictureURL: "", // 欢迎消息没有图片
		Role:       RoleAI,
		Time:       time.Now(),
	}

	if err := db.Create(&welcomeMessage).Error; err != nil {
		utils.LogError("创建欢迎消息失败: %v", err)
		return fmt.Errorf("创建欢迎消息失败: %v", err)
	}

	utils.LogInfo("欢迎消息创建成功 - 会话ID: %d", dialogID)
	return nil
}

// createDialogWithWelcomeMessage 创建会话并添加欢迎消息
func createDialogWithWelcomeMessage(tx *gorm.DB, userID, characterID uint, characterName string) (*models.Dialog, error) {
	// 创建新会话
	dialog, err := createDialogSafely(tx, userID, characterID)
	if err != nil {
		return nil, err
	}

	// 创建初始欢迎消息（使用事务中的数据库连接）
	if err := createWelcomeMessageWithDB(tx, dialog.ID, characterName); err != nil {
		return nil, err
	}

	return dialog, nil
}

// GetDialog 获取对话（如果不存在则创建）
func GetDialog(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID := c.GetUint("user_id")
	characterIDStr := c.Query("character_id")

	// 验证角色ID参数
	characterID, err := validateCharacterID(characterIDStr)
	if err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": err.Error(),
		})
		return
	}

	// 获取角色信息
	character, err := getCharacter(characterID)
	if err != nil {
		utils.LogError("角色不存在 - 角色ID: %d", characterID)
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
		})
		return
	}

	utils.LogInfo("获取角色成功 - 角色名称: %s, 角色ID: %d", character.Name, character.ID)

	// 查找会话
	var dialog models.Dialog
	if err := database.DB.Where("user_id = ? AND character_id = ?", userID, characterID).First(&dialog).Error; err != nil {
		utils.LogInfo("会话不存在，创建新会话 - 用户ID: %d, 角色ID: %d", userID, characterID)

		// 使用事务来防止并发创建重复会话
		tx := database.DB.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 在事务中再次检查会话是否存在
		var existingDialog models.Dialog
		if err := tx.Where("user_id = ? AND character_id = ?", userID, characterID).First(&existingDialog).Error; err == nil {
			// 会话已存在，使用现有会话
			dialog = existingDialog
			utils.LogInfo("在事务中发现现有会话 - 会话ID: %d", dialog.ID)
		} else {
			// 创建新会话
			dialogPtr, err := createDialogWithWelcomeMessage(tx, userID, characterID, character.Name)
			if err != nil {
				tx.Rollback()
				c.JSON(StatusInternalServerError, gin.H{
					"code":    CodeInternalError,
					"message": MessageCreateDialogError,
				})
				return
			}
			dialog = *dialogPtr
		}

		// 提交事务
		if err := tx.Commit().Error; err != nil {
			utils.LogError("提交事务失败: %v", err)
			c.JSON(StatusInternalServerError, gin.H{
				"code":    CodeInternalError,
				"message": MessageCreateDialogError,
			})
			return
		}

		// 清除会话列表缓存
		clearConversationCachesSafely(userID)
	} else {
		utils.LogInfo("找到现有会话 - 会话ID: %d", dialog.ID)
	}

	// 获取会话中的所有消息（过滤掉语音消息和已删除的消息）
	var messages []models.Message
	if err := database.DB.Where("dialog_id = ? AND is_voice = ? AND is_deleted = ?", dialog.ID, IsVoiceNo, IsDeletedNo).
		Order("time ASC").
		Find(&messages).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageGetDialogError,
		})
		return
	}

	// 构建响应数据
	var response []gin.H
	for _, message := range messages {
		response = append(response, gin.H{
			"id":          message.ID,
			"role":        message.Role,
			"content":     message.Content,
			"is_voice":    message.IsVoice,
			"picture_url": message.PictureURL, // 新增：返回图片URL
			"time":        message.Time,
		})
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageSuccess,
		"data": gin.H{
			"dialog_id": dialog.ID, // 新增：返回dialog_id
			"messages":  response,
			"character": gin.H{
				"id":         character.ID,
				"name":       character.Name,
				"avatar_url": character.AvatarURL,
			},
		},
	})
}

// GetUserConversations 获取用户会话列表
func GetUserConversations(c *gin.Context) {
	userID := c.GetUint("user_id")

	// 尝试从缓存获取数据
	cacheSvc := utils.NewCacheService()
	cacheKey := utils.GetConversationsKey(userID)

	var cachedResponse gin.H
	if err := cacheSvc.Get(cacheKey, &cachedResponse); err == nil {
		// 缓存命中，直接返回
		utils.LogInfo("🎯 [缓存命中] 会话列表 - 用户ID: %d, 键: %s", userID, cacheKey)
		c.JSON(StatusOK, cachedResponse)
		return
	}

	// 缓存未命中，从数据库获取数据
	utils.LogInfo("💾 [缓存未命中] 会话列表 - 用户ID: %d, 键: %s, 从数据库查询", userID, cacheKey)

	// 获取用户的所有会话（按置顶和时间排序）
	var conversationList []gin.H
	rows, err := database.DB.Raw(`
		SELECT 
			c.id as character_id,
			c.name as character_name,
			c.description as character_description,
			c.avatar_url as character_avatar,
			d.is_top,
			(SELECT content FROM messages 
			 WHERE dialog_id = d.id AND is_voice = ?
			 ORDER BY time DESC LIMIT 1) as last_message,
			d.updated_at as last_time
		FROM dialogs d
		INNER JOIN characters c ON d.character_id = c.id
		WHERE d.user_id = ?
		ORDER BY d.is_top DESC, d.updated_at DESC
	`, IsVoiceNo, userID).Rows()

	if err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageGetConversationError,
		})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var characterID uint
		var characterName, characterDescription, characterAvatar, lastMessage, isTop string
		var lastTime time.Time

		if err := rows.Scan(&characterID, &characterName, &characterDescription, &characterAvatar, &isTop, &lastMessage, &lastTime); err != nil {
			utils.LogWarn("⚠️ 扫描行数据失败: %v", err)
			continue
		}

		conversationList = append(conversationList, gin.H{
			"character_id":          characterID,
			"character_name":        characterName,
			"character_description": characterDescription,
			"character_avatar":      characterAvatar,
			"last_message":          lastMessage,
			"last_time":             lastTime,
			"is_top":                isTop,
		})
	}

	result := gin.H{
		"code":    CodeSuccess,
		"message": MessageSuccess,
		"data": gin.H{
			"conversations": conversationList,
		},
	}

	// 缓存结果
	if err := cacheSvc.Set(cacheKey, result, utils.ConversationsExpiration); err != nil {
		utils.LogError("⚠️ [缓存设置失败] 会话列表 - 用户ID: %d, 键: %s, 错误: %v", userID, cacheKey, err)
	} else {
		utils.LogInfo("💾 [缓存设置成功] 会话列表 - 用户ID: %d, 键: %s, 会话数量: %d", userID, cacheKey, len(conversationList))
	}

	c.JSON(StatusOK, result)
}

// PinConversation 置顶/取消置顶会话
func PinConversation(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID := c.GetUint("user_id")

	var req struct {
		CharacterID uint `json:"character_id" binding:"required"`
		IsTop       bool `json:"is_top"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 查找会话
	var dialog models.Dialog
	if err := database.DB.Where("user_id = ? AND character_id = ?", userID, req.CharacterID).First(&dialog).Error; err != nil {
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageDialogNotFound,
		})
		return
	}

	// 更新置顶状态
	isTopStr := IsTopNo
	if req.IsTop {
		isTopStr = IsTopYes
	}

	dialog.IsTop = isTopStr
	dialog.UpdatedAt = time.Now()

	if err := database.DB.Save(&dialog).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessagePinError,
		})
		return
	}

	// 清除会话列表缓存
	clearConversationCachesSafely(userID)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageOperationSuccess,
	})
}

// DeleteConversation 删除会话
func DeleteConversation(c *gin.Context) {
	userID := c.GetUint("user_id")
	characterIDStr := c.Param("characterId")
	var characterID uint
	if id, err := strconv.ParseUint(characterIDStr, 10, 32); err == nil {
		characterID = uint(id)
	} else {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 查找会话
	var dialog models.Dialog
	if err := database.DB.Where("user_id = ? AND character_id = ?", userID, characterID).First(&dialog).Error; err != nil {
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageDialogNotFound,
		})
		return
	}

	// 删除会话相关的所有消息
	if err := database.DB.Where("dialog_id = ?", dialog.ID).Delete(&models.Message{}).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageDeleteMessageError,
		})
		return
	}

	// 删除会话
	if err := database.DB.Delete(&dialog).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageDeleteDialogError,
		})
		return
	}

	// 清除会话列表缓存
	clearConversationCachesSafely(userID)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageDeleteSuccess,
	})
}

// UploadImages 上传图片
func UploadImages(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID := c.GetUint("user_id")

	// 获取上传的文件
	form, err := c.MultipartForm()
	if err != nil {
		utils.LogError("获取上传文件失败: %v", err)
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageGetUploadFilesError,
		})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageSelectImageFiles,
		})
		return
	}

	// 检查图片数量限制
	if len(files) > MaxImageCount {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageTooManyImages,
		})
		return
	}

	// 上传图片到OSS
	ossService := services.GetOSSService()
	if ossService == nil {
		utils.LogWarn("OSS服务未初始化")
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageImageServiceUnavailable,
		})
		return
	}

	imageUrls, err := ossService.UploadImages(files)
	if err != nil {
		utils.LogError("上传图片失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageUploadError,
		})
		return
	}

	utils.LogInfo("用户 %d 上传了 %d 张图片", userID, len(imageUrls))

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageUploadSuccess,
		"data": gin.H{
			"image_urls": imageUrls,
		},
	})
}

// SaveCallDurationMessage 新增：接收通话时长消息
func SaveCallDurationMessage(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID := c.GetUint("user_id")

	var req struct {
		DialogID    uint   `json:"dialog_id"`
		CharacterID uint   `json:"character_id"` // 新增：角色ID
		Content     string `json:"content"`
		IsVoice     string `json:"is_voice"`
		Role        string `json:"role"`
		Time        string `json:"time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 如果 dialog_id 为 0，需要先创建或查找会话
	var dialogID uint
	if req.DialogID == 0 {
		// 查找是否已有会话
		var dialog models.Dialog
		result := database.DB.Where("user_id = ? AND character_id = ?", userID, req.CharacterID).First(&dialog)
		if result.Error == nil {
			// 已有会话，使用现有会话ID
			dialogID = dialog.ID
			utils.LogInfo("找到现有会话 - 会话ID: %d", dialogID)
		} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// 没有会话，创建新会话
			newDialog, err := createDialogWithoutTransaction(userID, req.CharacterID)
			if err != nil {
				c.JSON(StatusInternalServerError, gin.H{
					"code":    CodeInternalError,
					"message": MessageCreateDialogError,
				})
				return
			}
			dialog = *newDialog
			dialogID = dialog.ID
		} else {
			utils.LogError("查找会话失败: %v", result.Error)
			c.JSON(StatusInternalServerError, gin.H{
				"code":    CodeInternalError,
				"message": MessageFindDialogError,
			})
			return
		}
	} else {
		dialogID = req.DialogID
	}

	msg := models.Message{
		DialogID:   dialogID,
		Content:    req.Content,
		PictureURL: "", // 通话时长消息没有图片
		IsVoice:    req.IsVoice,
		IsDeleted:  IsDeletedNo, // 确保设置为未删除
		Role:       req.Role,
		Time:       time.Now(),
	}

	if err := database.DB.Create(&msg).Error; err != nil {
		utils.LogError("创建通话时长消息失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageSaveCallDurationError,
		})
		return
	}

	// 更新会话时间
	var dialog models.Dialog
	if err := database.DB.First(&dialog, dialogID).Error; err == nil {
		dialog.UpdatedAt = time.Now()
		database.DB.Save(&dialog)
	}

	// 清除会话列表缓存
	clearConversationCachesSafely(userID)

	utils.LogInfo("通话时长消息保存成功 - 会话ID: %d, 内容长度: %d", dialogID, len(req.Content))
	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageSaveSuccess,
	})
}

// DeleteMessage 删除消息（逻辑删除）
func DeleteMessage(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID := c.GetUint("user_id")

	var req struct {
		MessageID uint `json:"message_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("参数绑定失败: %v", err)
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	utils.LogInfo("用户 %d 请求删除消息 %d", userID, req.MessageID)

	// 查找消息
	var message models.Message
	if err := database.DB.Where("id = ? AND is_deleted = ?", req.MessageID, IsDeletedNo).First(&message).Error; err != nil {
		utils.LogWarn("消息不存在或已被删除 - 消息ID: %d, 错误: %v", req.MessageID, err)
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageMessageNotFound,
		})
		return
	}

	utils.LogInfo("找到消息 - ID: %d, 角色: %s, 内容长度: %d", message.ID, message.Role, len(message.Content))

	// 执行逻辑删除
	if err := database.DB.Model(&message).Update("is_deleted", IsDeletedYes).Error; err != nil {
		utils.LogError("删除消息失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageDeleteMessageError,
		})
		return
	}

	utils.LogInfo("消息删除成功 - 消息ID: %d", req.MessageID)

	// 清除会话列表缓存
	clearConversationCachesSafely(userID)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageDeleteSuccess,
	})
}

// SendChatMessage 发送聊天消息
func SendChatMessage(c *gin.Context) {
	// 从认证中间件获取用户ID
	userID := c.GetUint("user_id")

	var req ChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("参数绑定失败: %v", err)
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 脱敏记录：不记录具体的用户消息内容
	utils.LogInfo("收到聊天消息 - 用户ID: %d, 角色ID: %d, 消息长度: %d, 图片数量: %d",
		userID, req.CharacterID, len(req.Message), len(req.ImageUrls))

	// 验证消息内容
	if err := validateChatRequest(req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": err.Error(),
		})
		return
	}

	// 获取角色信息
	character, err := getCharacter(req.CharacterID)
	if err != nil {
		utils.LogError("获取角色失败 - 角色ID: %d, 错误: %v", req.CharacterID, err)
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
		})
		return
	}

	utils.LogInfo("获取角色成功 - 角色名称: %s, 提示词: %s", character.Name, character.Prompt)

	// 查找或创建会话
	dialog, err := findOrCreateDialog(userID, req.CharacterID)
	if err != nil {
		utils.LogError("查找或创建会话失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageCreateDialogError,
		})
		return
	}

	// 创建用户消息
	userMessage := createUserMessage(dialog.ID, req.Message, req.ImageUrls)

	// 生成AI回复
	aiReply := generateAIReply(dialog.ID, character.Prompt, req.Message, req.ImageUrls)

	// 创建AI消息
	aiMessage := createAIMessage(dialog.ID, aiReply)

	// 更新会话时间
	updateDialogTime(dialog)

	// 清除会话列表缓存
	clearConversationCachesSafely(userID)

	response := ChatMessageResponse{
		Reply: aiReply,
		// 不再生成AudioURL，因为语音聊天中会直接提供
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageSendSuccess,
		"data": gin.H{
			"reply":     response.Reply,
			"audio_url": response.AudioURL,
			"user_message": gin.H{
				"id":          userMessage.ID,
				"role":        userMessage.Role,
				"content":     userMessage.Content,
				"picture_url": userMessage.PictureURL,
				"time":        userMessage.Time,
			},
			"ai_message": gin.H{
				"id":          aiMessage.ID,
				"role":        aiMessage.Role,
				"content":     aiMessage.Content,
				"picture_url": aiMessage.PictureURL,
				"time":        aiMessage.Time,
			},
		},
	})
}
