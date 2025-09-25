package handlers

import (
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/services"
	"strings"
	"time"

	"ai-celebrity-simulator/utils"

	"github.com/gin-gonic/gin"
)

// 响应消息常量
const (
	MessageLoginSuccess  = "登录成功"
	MessageLogoutSuccess = "退出登录成功"

	MessageWeChatLoginError    = "微信登录失败"
	MessageUserCreateError     = "用户创建失败"
	MessageTokenCreateError    = "Token创建失败"
	MessageTokenFormatError    = "token格式错误"
	MessageLogoutError         = "退出登录失败"
	MessageUserNotFound        = "用户不存在"
	MessageDatabaseUpdateError = "数据库更新失败"
	MessageUploadAvatarError   = "上传头像失败"
)

// DateFormat 时间格式常量
const (
	DateFormat = "2006-01-02"
	// TimeFormat已在character.go中定义
)

// UserFolder 文件夹名称常量
const (
	UserFolder = "user"
)

// 临时目录相关常量
const (
	WxFilePrefix = "wxfile://"
	TempDir      = "/tmp/"
	DataDir      = "/data/"
)

// TemporaryFilePrefixes 文件路径前缀常量
var TemporaryFilePrefixes = []string{
	WxFilePrefix,
	TempDir,
	DataDir,
}

// clearUserCachesSafely 安全地清除用户缓存，处理错误但不中断流程
func clearUserCachesSafely(userID uint) {
	if err := utils.ClearUserCaches(userID); err != nil {
		utils.LogWarn("⚠️ 清除用户缓存失败 - 用户ID: %d, 错误: %v", userID, err)
	}
}

type LoginRequest struct {
	Code string `json:"code" binding:"required"`
}

type LoginResponse struct {
	UserID   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Token    string `json:"token"`
}

// UserLogin 用户登录
func UserLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 调用微信登录
	utils.LogInfo("开始微信登录，code: %s", req.Code)
	wechatResp, err := services.WeChatLogin(req.Code)
	if err != nil {
		utils.LogError("微信登录失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageWeChatLoginError,
		})
		return
	}

	// 创建或获取用户
	user, _, err := services.CreateOrGetUser(wechatResp.OpenID)
	if err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageUserCreateError,
		})
		return
	}

	// 创建用户token
	token, err := services.CreateUserToken(user.ID)
	if err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageTokenCreateError,
		})
		return
	}

	response := LoginResponse{
		UserID:   user.ID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Token:    token,
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageLoginSuccess,
		"data":    response,
	})
}

// GetWeChatUserInfoByCode 获取微信用户信息（通过前端授权）
func GetWeChatUserInfoByCode(c *gin.Context) {
	userID := c.GetUint("user_id")
	utils.LogInfo("当前用户uid: %d", userID)
	var req struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 处理头像：如果是微信临时文件，需要前端先上传到OSS，否则直接存url
	avatarUrl := req.Avatar
	if len(avatarUrl) > 0 {
		for _, prefix := range TemporaryFilePrefixes {
			if strings.HasPrefix(avatarUrl, prefix) {
				// 理论上前端应先上传，后端兜底不处理本地文件路径
				avatarUrl = ""
				break
			}
		}
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeBadRequest,
			"message": MessageUserNotFound,
		})
		return
	}

	updates := make(map[string]interface{})
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if avatarUrl != "" {
		updates["avatar"] = avatarUrl
	}
	updates["updated_at"] = time.Now()
	database.DB.Model(&user).Updates(updates)
	database.DB.First(&user, userID)

	// 清除用户相关缓存
	clearUserCachesSafely(userID)
	createdAt := user.CreatedAt.Format(DateFormat)
	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageSuccess,
		"data": gin.H{
			"user_info": gin.H{
				"nickname":   user.Nickname,
				"avatar":     user.Avatar,
				"created_at": createdAt,
			},
		},
	})
}

// UpdateUserInfo 更新用户信息
func UpdateUserInfo(c *gin.Context) {
	userID := c.GetUint("user_id")
	utils.LogInfo("开始更新用户信息 - 用户ID: %d", userID)

	var req struct {
		Nickname string `json:"nickname"`
		Avatar   string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("参数绑定失败: %v", err)
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": MessageParamError,
		})
		return
	}

	// 脱敏记录：不记录具体的昵称和头像内容
	utils.LogInfo("接收到的更新请求 - 昵称长度: %d, 头像长度: %d", len(req.Nickname), len(req.Avatar))

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		utils.LogError("用户不存在 - 用户ID: %d, 错误: %v", userID, err)
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeNotFound,
			"message": MessageUserNotFound,
		})
		return
	}

	utils.LogInfo("更新前用户信息 - 用户ID: %d", userID)

	// 允许修改昵称和头像
	if req.Nickname != "" {
		user.Nickname = req.Nickname
		utils.LogInfo("更新昵称 - 用户ID: %d, 昵称长度: %d", userID, len(req.Nickname))
	}

	avatar := req.Avatar
	for _, prefix := range TemporaryFilePrefixes {
		if strings.HasPrefix(avatar, prefix) {
			utils.LogInfo("检测到临时文件路径，清空头像 - 用户ID: %d", userID)
			avatar = ""
			break
		}
	}
	if avatar != "" {
		user.Avatar = avatar
		utils.LogInfo("更新头像 - 用户ID: %d, 头像长度: %d", userID, len(avatar))
	}

	user.UpdatedAt = time.Now()

	// 更新用户信息
	if err := database.DB.Save(&user).Error; err != nil {
		utils.LogError("数据库更新失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageDatabaseUpdateError,
		})
		return
	}

	utils.LogInfo("更新后用户信息 - 用户ID: %d", userID)

	// 清除用户相关缓存
	clearUserCachesSafely(userID)

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageUpdateSuccess,
		"data": gin.H{
			"user": gin.H{
				"id":         user.ID,
				"nickname":   user.Nickname,
				"avatar":     user.Avatar,
				"created_at": user.CreatedAt.Format(TimeFormat),
				"updated_at": user.UpdatedAt.Format(TimeFormat),
			},
		},
	})
}

// UserLogout 用户退出登录
func UserLogout(c *gin.Context) {
	// 获取当前请求的token
	authHeader := c.GetHeader("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" || token == authHeader {
		c.JSON(StatusUnauthorized, gin.H{
			"code":    CodeUnauthorized,
			"message": MessageTokenFormatError,
		})
		return
	}

	// 删除当前token
	if err := services.DeleteToken(token); err != nil {
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageLogoutError,
		})
		return
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": MessageLogoutSuccess,
	})
}

// GetUserInfo 获取当前登录用户信息
func GetUserInfo(c *gin.Context) {
	userID := c.GetUint("user_id")

	// 尝试从缓存获取数据
	cacheSvc := utils.NewCacheService()
	cacheKey := utils.GetUserInfoKey(userID)

	var cachedResponse gin.H
	if err := cacheSvc.Get(cacheKey, &cachedResponse); err == nil {
		// 缓存命中，直接返回
		utils.LogInfo("🎯 [缓存命中] 用户信息 - 用户ID: %d, 键: %s", userID, cacheKey)
		c.JSON(StatusOK, cachedResponse)
		return
	}

	// 缓存未命中，从数据库获取数据
	utils.LogInfo("💾 [缓存未命中] 用户信息 - 用户ID: %d, 键: %s, 从数据库查询", userID, cacheKey)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(StatusNotFound, gin.H{
			"code":    CodeBadRequest,
			"message": MessageUserNotFound,
		})
		return
	}

	result := gin.H{
		"code":    CodeSuccess,
		"message": "获取成功",
		"data": gin.H{
			"user": gin.H{
				"id":         user.ID,
				"nickname":   user.Nickname,
				"avatar":     user.Avatar,
				"created_at": user.CreatedAt.Format(TimeFormat),
				"updated_at": user.UpdatedAt.Format(TimeFormat),
			},
		},
	}

	// 缓存结果
	if err := cacheSvc.Set(cacheKey, result, utils.UserInfoExpiration); err != nil {
		utils.LogError("⚠️ [缓存设置失败] 用户信息 - 用户ID: %d, 键: %s, 错误: %v", userID, cacheKey, err)
	} else {
		utils.LogInfo("💾 [缓存设置成功] 用户信息 - 用户ID: %d, 键: %s, 用户昵称: %s", userID, cacheKey, user.Nickname)
	}

	c.JSON(StatusOK, result)
}

// UploadUserAvatar 上传用户头像到OSS
func UploadUserAvatar(c *gin.Context) {
	userID := c.GetUint("user_id")
	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(StatusBadRequest, gin.H{
			"code":    CodeBadRequest,
			"message": "请选择头像文件",
		})
		return
	}
	avatarURL, err := services.UploadFileToOSS(file, UserFolder)
	if err != nil {
		utils.LogError("上传用户头像失败: %v", err)
		c.JSON(StatusInternalServerError, gin.H{
			"code":    CodeInternalError,
			"message": MessageUploadAvatarError,
		})
		return
	}
	// 更新数据库
	var user models.User
	if err := database.DB.First(&user, userID).Error; err == nil {
		user.Avatar = avatarURL
		database.DB.Save(&user)

		// 清除用户相关缓存
		clearUserCachesSafely(userID)
	}

	c.JSON(StatusOK, gin.H{
		"code":    CodeSuccess,
		"message": "头像上传成功",
		"data": gin.H{
			"avatar_url": avatarURL,
		},
	})
}
