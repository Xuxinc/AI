package handlers

import (
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/services"
	"errors"
	"net/http"
	"strconv"
	"time"

	"ai-celebrity-simulator/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HTTP状态码常量
const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusNotFound            = http.StatusNotFound
	StatusConflict            = http.StatusConflict
	StatusInternalServerError = http.StatusInternalServerError
)

// 业务状态码常量
const (
	CodeSuccess       = 200
	CodeBadRequest    = 400
	CodeUnauthorized  = 401
	CodeForbidden     = 403
	CodeNotFound      = 404
	CodeConflict      = 409
	CodeInternalError = 500
)

// 响应消息常量
const (
	MessageSuccess             = "获取成功"
	MessageCreateSuccess       = "角色生成成功"
	MessageCustomCreateSuccess = "角色创建成功"
	MessageUpdateSuccess       = "更新成功"
	MessageDeleteSuccess       = "删除成功"
	MessagePublicSuccess       = "角色公开成功"
	MessageUploadSuccess       = "头像上传成功"
	MessageOperationSuccess    = "操作成功"

	MessageParamError             = "参数错误"
	MessageCharacterNotFound      = "角色不存在"
	MessageCharacterExists        = "角色已存在"
	MessageDatabaseQueryError     = "数据库查询失败"
	MessageGenerateCharacterError = "生成角色失败"
	MessageSaveCharacterError     = "保存角色失败"
	MessageUploadAvatarFirst      = "请先上传头像"
	MessageInvalidCharacterID     = "无效的角色ID"
	MessageCharacterIDFormatError = "角色ID格式错误"
	MessageUpdateError            = "更新失败"
	MessageDeleteError            = "删除角色失败"
	MessagePublicError            = "公开角色失败"
	MessageUploadError            = "上传头像失败"
	MessagePermissionDenied       = "无权限删除此角色"
	MessagePermissionDeniedPublic = "无权限公开此角色"
	MessageOnlyCustomCharacter    = "只能公开自己创建的角色"
	MessageHasDialogs             = "该角色存在相关对话，请先删除对话后再删除角色"
	MessageCheckDialogError       = "检查对话失败"
	MessagePromptRequired         = "提示词不能为空"
	MessageInvalidVoiceModel      = "无效的音色模型"
	MessageSelectAvatarFile       = "请选择头像文件"

	// 搜索相关消息
	MessageCharacterNameEmpty      = "角色名称不能为空"
	MessageCharacterExistsInSearch = "角色存在"
)

// 数据库字段值常量
const (
	IsCreatedByUserNo  = "no"
	IsCreatedByUserYes = "yes"
)

// TimeFormat 时间格式常量
const (
	TimeFormat = "2006-01-02 15:04:05"
)

// CustomAvatarFolder 文件夹名称常量
const (
	CustomAvatarFolder = "custom-avatars"
)

// ValidVoiceModels 音色模型常量
var ValidVoiceModels = []string{
	// 客服
	"longyingcui", "longyingda", "longyingjing", "longyingyan", "longyingtian", "longyingbing", "longyingtao", "longyingling",
	// 语音助手
	"longyumi_v2", "longxiaochun_v2", "longxiaoxia_v2",
	// 直播
	"longanran", "longanxuan",
	// 有声书
	"longsanshu", "longxiu_v2", "longmiao_v2", "longyue_v2", "longnan_v2", "longyuan_v2",
	// 社交陪伴
	"longanrou", "longqiang_v2", "longhan_v2", "longxing_v2", "longhua_v2", "longwan_v2", "longcheng_v2", "longfeifei_v2", "longxiaocheng_v2", "longzhe_v2", "longyan_v2", "longtian_v2", "longze_v2", "longshao_v2", "longhao_v2", "kabuleshen_v2",
	// 童声
	"longjielidou_v2", "longling_v2", "longke_v2", "longxian_v2",
	// 方言
	"longlaotie_v2", "longjiayi_v2", "longtao_v2",
	// 诗词朗诵
	"longfei_v2", "libai_v2", "longjin_v2",
	// 新闻播报
	"longshu_v2", "loongbella_v2", "longshuo_v2", "longxiaobai_v2", "longjing_v2", "loongstella_v2",
}

// GetCharacters 获取名人角色列表
func GetCharacters(c *gin.Context) {
	// 获取用户ID（如果已登录）
	userID := c.GetUint("user_id")

	// 尝试从缓存获取数据
	cacheSvc := utils.NewCacheService()
	cacheKey := utils.GetCharactersKey(userID)

	var cachedResponse gin.H
	if err := cacheSvc.Get(cacheKey, &cachedResponse); err == nil {
		// 缓存命中，直接返回
		utils.LogInfo("🎯 [缓存命中] 角色列表 - 用户ID: %d, 键: %s", userID, cacheKey)
		c.JSON(StatusOK, cachedResponse)
		return
	}

	// 缓存未命中，从数据库获取数据
	utils.LogInfo("💾 [缓存未命中] 角色列表 - 用户ID: %d, 键: %s, 从数据库查询", userID, cacheKey)

	var characters []models.Character
	var total int64

	utils.LogInfo("当前用户uid: %d", userID)
	query := database.DB.Model(&models.Character{})
	if userID != 0 {
		query = query.Where("is_created_by_user = ? OR (is_created_by_user = ? AND uid = ?)", IsCreatedByUserNo, IsCreatedByUserYes, userID)
	} else {
		query = query.Where("is_created_by_user = ?", IsCreatedByUserNo)
	}

	// 按更新时间倒序排序
	query = query.Order("updated_at DESC")

	// 获取总数
	query.Count(&total)

	// 获取所有数据
	if err := query.Find(&characters).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{"error": "获取角色列表失败"})
		return
	}

	var response []gin.H
	for _, character := range characters {
		response = append(response, gin.H{
			"id":                 character.ID,
			"name":               character.Name,
			"description":        character.Description,
			"avatar_url":         character.AvatarURL,
			"is_created_by_user": character.IsCreatedByUser,
			"uid":                character.UID,
			"voice_model":        character.VoiceModel,
			"updated_at":         character.UpdatedAt.Format(TimeFormat),
		})
	}

	result := gin.H{
		"code":    CodeSuccess,
		"message": MessageSuccess,
		"data": gin.H{
			"characters": response,
			"total":      total,
		},
	}

	// 缓存结果
	if err := cacheSvc.Set(cacheKey, result, utils.CharactersExpiration); err != nil {
		utils.LogError("⚠️ [缓存设置失败] 角色列表 - 用户ID: %d, 键: %s, 错误: %v", userID, cacheKey, err)
	} else {
		utils.LogInfo("💾 [缓存设置成功] 角色列表 - 用户ID: %d, 键: %s, 角色数量: %d", userID, cacheKey, len(response))
	}

	c.JSON(StatusOK, result)
}

// SearchCharacter 搜索角色
func SearchCharacter(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageCharacterNameEmpty,
		})
		return
	}

	// 尝试从缓存获取数据
	cacheSvc := utils.NewCacheService()
	cacheKey := utils.GetSearchKey(name)

	var cachedResponse gin.H
	if err := cacheSvc.Get(cacheKey, &cachedResponse); err == nil {
		// 缓存命中，直接返回
		utils.LogInfo("🎯 [缓存命中] 角色搜索 - 键: %s", cacheKey)
		c.JSON(StatusOK, cachedResponse)
		return
	}

	// 缓存未命中，从数据库搜索
	utils.LogInfo("💾 [缓存未命中] 角色搜索 - 键: %s, 从数据库查询", cacheKey)

	var character models.Character
	result := database.DB.Where("name = ?", name).First(&character)

	var response gin.H
	if result.Error != nil {
		// 角色不存在
		response = gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
			"data": gin.H{
				"exists": false,
				"name":   name,
			},
		}
		utils.LogInfo("🔍 [搜索结果] 角色不存在 - 查询名称长度: %d", len(name))
	} else {
		// 角色存在
		response = gin.H{
			"code":    CodeSuccess,
			"message": MessageCharacterExistsInSearch,
			"data": gin.H{
				"exists": true,
				"character": gin.H{
					"id":          character.ID,
					"name":        character.Name,
					"description": character.Description,
					"avatar_url":  character.AvatarURL,
				},
			},
		}
		utils.LogInfo("🔍 [搜索结果] 角色存在 - 角色ID: %d, 名称长度: %d", character.ID, len(name))
	}

	// 缓存结果
	if err := cacheSvc.Set(cacheKey, response, utils.SearchExpiration); err != nil {
		utils.LogError("⚠️ [缓存设置失败] 角色搜索 - 键: %s, 错误: %v", cacheKey, err)
	} else {
		utils.LogInfo("💾 [缓存设置成功] 角色搜索 - 键: %s", cacheKey)
	}

	c.JSON(StatusOK, response)
}

// GenerateCelebrityCharacter 生成名人角色
func GenerateCelebrityCharacter(c *gin.Context) {
	// 获取用户ID
	userID := c.GetUint("user_id")

	var req services.CharacterGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 检查角色是否已存在
	var existingCharacter models.Character
	result := database.DB.Where("name = ?", req.Name).First(&existingCharacter)
	if result.Error == nil {
		// 角色已存在
		c.JSON(StatusConflict, gin.H{
			"code":    CodeConflict,
			"message": MessageCharacterExists,
		})
		return
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 其他数据库错误
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageDatabaseQueryError,
		})
		return
	}

	// 生成角色信息
	characterInfo, err := services.GenerateCelebrityCharacter(req.Name)
	if err != nil {
		utils.LogError("生成角色信息失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageGenerateCharacterError,
		})
		return
	}

	// 保存到数据库
	character := models.Character{
		Name:            characterInfo.Name,
		Description:     characterInfo.Description,
		Prompt:          characterInfo.Prompt,
		VoiceModel:      characterInfo.VoiceModel,
		AvatarURL:       characterInfo.AvatarURL,
		IsCreatedByUser: IsCreatedByUserNo, // 名人角色所有用户都能看到
		UID:             &userID,           // 记录创建者UID，但不影响可见性
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := database.DB.Create(&character).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageSaveCharacterError,
		})
		return
	}

	// 清除相关缓存
	if err := utils.ClearCharacterCaches(character.ID); err != nil {
		utils.LogWarn("⚠️ 清除角色缓存失败 - 角色ID: %d, 错误: %v", character.ID, err)
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageCreateSuccess,
		"data": gin.H{
			"id":          character.ID,
			"name":        character.Name,
			"description": character.Description,
			"avatar_url":  character.AvatarURL,
		},
	})
}

// GenerateCustomCharacter 生成自定义角色
func GenerateCustomCharacter(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req services.CharacterGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}
	// 检查角色是否已存在
	var existingCharacter models.Character
	result := database.DB.Where("name = ?", req.Name).First(&existingCharacter)
	if result.Error == nil {
		c.JSON(StatusConflict, gin.H{
			"code":    CodeConflict,
			"message": MessageCharacterExists,
		})
		return
	} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageDatabaseQueryError,
		})
		return
	}
	// 角色不存在，继续创建流程
	if req.AvatarURL == "" {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageUploadAvatarFirst,
		})
		return
	}
	characterInfo, err := services.GenerateCustomCharacter(req.Name, req.Description, req.AvatarURL)
	if err != nil {
		utils.LogError("生成自定义角色失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageGenerateCharacterError,
		})
		return
	}
	character := models.Character{
		Name:            characterInfo.Name,
		Description:     characterInfo.Description,
		Prompt:          characterInfo.Prompt,
		VoiceModel:      characterInfo.VoiceModel,
		AvatarURL:       characterInfo.AvatarURL,
		IsCreatedByUser: IsCreatedByUserYes,
		UID:             &userID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := database.DB.Create(&character).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageSaveCharacterError,
		})
		return
	}

	// 清除相关缓存
	if err := utils.ClearCharacterCaches(character.ID); err != nil {
		utils.LogWarn("⚠️ 清除角色缓存失败 - 角色ID: %d, 错误: %v", character.ID, err)
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageCustomCreateSuccess,
		"data": gin.H{
			"id":          character.ID,
			"name":        character.Name,
			"description": character.Description,
			"avatar_url":  character.AvatarURL,
		},
	})
}

// UploadCustomCharacterAvatar 上传自定义角色头像
func UploadCustomCharacterAvatar(c *gin.Context) {
	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageSelectAvatarFile,
		})
		return
	}
	avatarURL, err := services.UploadFileToOSS(file, CustomAvatarFolder)
	if err != nil {
		utils.LogError("上传头像失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageUploadError,
		})
		return
	}
	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageUploadSuccess,
		"data": gin.H{
			"avatar_url": avatarURL,
		},
	})
}

// GetCharacterDetail 获取角色详情
func GetCharacterDetail(c *gin.Context) {
	// 获取角色ID
	characterIDStr := c.Param("id")
	characterID, err := strconv.ParseUint(characterIDStr, 10, 32)
	if err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageInvalidCharacterID,
		})
		return
	}

	// 尝试从缓存获取数据
	cacheSvc := utils.NewCacheService()
	cacheKey := utils.GetCharacterDetailKey(uint(characterID))

	var cachedResponse gin.H
	if err := cacheSvc.Get(cacheKey, &cachedResponse); err == nil {
		// 缓存命中，直接返回
		utils.LogInfo("🎯 [缓存命中] 角色详情 - 角色ID: %d, 键: %s", characterID, cacheKey)
		c.JSON(StatusOK, cachedResponse)
		return
	}

	// 缓存未命中，从数据库获取数据
	utils.LogInfo("💾 [缓存未命中] 角色详情 - 角色ID: %d, 键: %s, 从数据库查询", characterID, cacheKey)

	var character models.Character
	if err := database.DB.First(&character, characterID).Error; err != nil {
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
		})
		return
	}

	result := gin.H{
		"code":    CodeSuccess,
		"message": MessageSuccess,
		"data": gin.H{
			"id":          character.ID,
			"name":        character.Name,
			"description": character.Description,
			"prompt":      character.Prompt,
			"voice_model": character.VoiceModel,
			"avatar_url":  character.AvatarURL,
			"created_at":  character.CreatedAt.Format(TimeFormat),
			"updated_at":  character.UpdatedAt.Format(TimeFormat),
		},
	}

	// 缓存结果
	if err := cacheSvc.Set(cacheKey, result, utils.CharacterDetailExpiration); err != nil {
		utils.LogError("⚠️ [缓存设置失败] 角色详情 - 角色ID: %d, 键: %s, 错误: %v", characterID, cacheKey, err)
	} else {
		utils.LogInfo("💾 [缓存设置成功] 角色详情 - 角色ID: %d, 键: %s", characterID, cacheKey)
	}

	c.JSON(StatusOK, result)
}

// UpdateCharacter 更新角色信息
func UpdateCharacter(c *gin.Context) {
	// 获取角色ID
	characterIDStr := c.Param("id")
	characterID, err := strconv.ParseUint(characterIDStr, 10, 32)
	if err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageInvalidCharacterID,
		})
		return
	}

	// 解析请求体
	var updateData struct {
		Prompt     string `json:"prompt"`
		VoiceModel string `json:"voice_model"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 验证必填字段
	if updateData.Prompt == "" {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessagePromptRequired,
		})
		return
	}

	// 验证音色模型（支持预设音色和自定义音色）
	isValidVoiceModel := false
	for _, model := range ValidVoiceModels {
		if model == updateData.VoiceModel {
			isValidVoiceModel = true
			break
		}
	}

	// 如果不是预设音色，检查是否为自定义音色（以特定格式开头的voice_id）
	if !isValidVoiceModel && len(updateData.VoiceModel) > 0 {
		// 自定义音色通常以特定格式的voice_id形式存在
		isValidVoiceModel = true
	}

	if !isValidVoiceModel {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageInvalidVoiceModel,
		})
		return
	}

	// 查询角色是否存在
	var character models.Character
	if err := database.DB.First(&character, characterID).Error; err != nil {
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
		})
		return
	}

	// 更新数据库
	character.Prompt = updateData.Prompt
	character.VoiceModel = updateData.VoiceModel
	character.UpdatedAt = time.Now()

	if err := database.DB.Save(&character).Error; err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageUpdateError,
		})
		return
	}

	// 清除相关缓存
	if err := utils.ClearCharacterCaches(uint(characterID)); err != nil {
		utils.LogWarn("⚠️ 清除角色缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageUpdateSuccess,
		"data":    character,
	})
}

// GetPublicCharacters 获取公开名人角色列表
func GetPublicCharacters(c *gin.Context) {
	// 尝试从缓存获取数据
	cacheSvc := utils.NewCacheService()
	cacheKey := utils.GetPublicCharactersKey()

	var cachedResponse gin.H
	if err := cacheSvc.Get(cacheKey, &cachedResponse); err == nil {
		// 缓存命中，直接返回
		utils.LogInfo("🎯 [缓存命中] 公开角色列表 - 键: %s", cacheKey)
		c.JSON(StatusOK, cachedResponse)
		return
	}

	// 缓存未命中，从数据库获取数据
	utils.LogInfo("💾 [缓存未命中] 公开角色列表 - 键: %s, 从数据库查询", cacheKey)

	var characters []models.Character
	database.DB.Where("is_created_by_user = ?", IsCreatedByUserNo).Order("updated_at DESC").Find(&characters)
	var response []gin.H
	for _, character := range characters {
		response = append(response, gin.H{
			"id":                 character.ID,
			"name":               character.Name,
			"description":        character.Description,
			"avatar_url":         character.AvatarURL,
			"is_created_by_user": character.IsCreatedByUser,
			"uid":                character.UID,
			"voice_model":        character.VoiceModel,
			"updated_at":         character.UpdatedAt.Format(TimeFormat),
		})
	}

	result := gin.H{
		"code":    CodeSuccess,
		"message": MessageSuccess,
		"data": gin.H{
			"characters": response,
			"total":      len(response),
		},
	}

	// 缓存结果
	if err := cacheSvc.Set(cacheKey, result, utils.PublicCharactersExpiration); err != nil {
		utils.LogError("⚠️ [缓存设置失败] 公开角色列表 - 键: %s, 错误: %v", cacheKey, err)
	} else {
		utils.LogInfo("💾 [缓存设置成功] 公开角色列表 - 键: %s, 角色数量: %d", cacheKey, len(response))
	}

	c.JSON(StatusOK, result)
}

// DeleteCharacter 删除名人角色
func DeleteCharacter(c *gin.Context) {
	// 获取用户ID
	userID := c.GetUint("user_id")
	characterIDStr := c.Param("id")

	characterID, err := strconv.ParseUint(characterIDStr, 10, 32)
	if err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeBadRequest,
			"message": MessageCharacterIDFormatError,
		})
		return
	}

	// 查找角色
	var character models.Character
	if err := database.DB.First(&character, characterID).Error; err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
		})
		return
	}

	// 检查权限：只有创建者才能删除
	if character.UID == nil || *character.UID != userID {
		c.JSON(StatusOK, gin.H{
			"code":    CodeForbidden,
			"message": MessagePermissionDenied,
		})
		return
	}

	// 检查是否有相关对话
	var dialogCount int64
	if err := database.DB.Model(&models.Dialog{}).Where("character_id = ?", characterID).Count(&dialogCount).Error; err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeInternalError,
			"message": MessageCheckDialogError,
		})
		return
	}

	if dialogCount > 0 {
		c.JSON(StatusOK, gin.H{
			"code":    CodeBadRequest,
			"message": MessageHasDialogs,
		})
		return
	}

	// 删除角色
	if err := database.DB.Delete(&character).Error; err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeInternalError,
			"message": MessageDeleteError,
		})
		return
	}

	// 清除相关缓存
	if err := utils.ClearCharacterCaches(uint(characterID)); err != nil {
		utils.LogWarn("⚠️ 清除角色缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
	}

	utils.LogInfo("用户 %d 删除了角色 %d", userID, characterID)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageDeleteSuccess,
	})
}

// PublicCharacter 公开角色
func PublicCharacter(c *gin.Context) {
	// 获取用户ID
	userID := c.GetUint("user_id")
	characterIDStr := c.Param("id")

	characterID, err := strconv.ParseUint(characterIDStr, 10, 32)
	if err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeBadRequest,
			"message": MessageCharacterIDFormatError,
		})
		return
	}

	// 查找角色
	var character models.Character
	if err := database.DB.First(&character, characterID).Error; err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeNotFound,
			"message": MessageCharacterNotFound,
		})
		return
	}

	// 检查权限：只有创建者才能公开
	if character.UID == nil || *character.UID != userID {
		c.JSON(StatusOK, gin.H{
			"code":    CodeForbidden,
			"message": MessagePermissionDeniedPublic,
		})
		return
	}

	// 检查是否为用户创建的角色
	if character.IsCreatedByUser != IsCreatedByUserYes {
		c.JSON(StatusOK, gin.H{
			"code":    CodeBadRequest,
			"message": MessageOnlyCustomCharacter,
		})
		return
	}

	// 更新角色为公开状态
	character.IsCreatedByUser = IsCreatedByUserNo
	character.UpdatedAt = time.Now()

	if err := database.DB.Save(&character).Error; err != nil {
		c.JSON(StatusOK, gin.H{
			"code":    CodeInternalError,
			"message": MessagePublicError,
		})
		return
	}

	// 清除相关缓存
	if err := utils.ClearCharacterCaches(uint(characterID)); err != nil {
		utils.LogWarn("⚠️ 清除角色缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
	}

	utils.LogInfo("用户 %d 公开了角色 %d", userID, characterID)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessagePublicSuccess,
	})
}
