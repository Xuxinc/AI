package services

import (
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/utils"
	"errors"
	"fmt"
	"time"
)

// 用户配置常量
const (
	DefaultNicknameLength = 6
	DefaultAvatarURL      = "https://www.dhs.tsinghua.edu.cn/wp-content/uploads/2025/03/2025031301575583.jpeg"
)

// 错误定义
var (
	ErrUserNotFound     = errors.New("用户不存在")
	ErrUserCreateFailed = errors.New("用户创建失败")
	ErrTokenNotFound    = errors.New("token不存在或已过期")
	ErrTokenStoreFailed = errors.New("token存储失败")
)

// CreateOrGetUser 创建或获取用户
func CreateOrGetUser(openid string) (*models.User, bool, error) {
	// 脱敏记录：不记录完整的OpenID
	utils.LogInfo("查找用户 - OpenID长度: %d", len(openid))

	// 验证输入参数
	if err := validateOpenID(openid); err != nil {
		utils.LogError("OpenID验证失败: %v", err)
		return nil, false, err
	}

	var user models.User
	result := database.DB.Where("openid = ?", openid).First(&user)

	if result.Error != nil {
		utils.LogInfo("用户不存在，开始创建新用户")
		return createNewUser(openid)
	}

	utils.LogInfo("用户已存在 - 用户ID: %d", user.ID)
	return &user, false, nil // 已存在用户
}

// validateOpenID 验证OpenID
func validateOpenID(openid string) error {
	if openid == "" {
		return errors.New("OpenID不能为空")
	}
	return nil
}

// createNewUser 创建新用户
func createNewUser(openid string) (*models.User, bool, error) {
	userID, err := utils.GenerateUserID()
	if err != nil {
		utils.LogError("生成用户ID失败: %v", err)
		return nil, false, fmt.Errorf("生成用户ID失败: %w", err)
	}

	// 生成默认昵称（使用openid后几位，但不记录具体内容）
	nickname := generateDefaultNickname(openid)

	user := models.User{
		ID:        userID,
		OpenID:    openid,
		Nickname:  nickname,
		Avatar:    DefaultAvatarURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := database.DB.Create(&user).Error; err != nil {
		utils.LogError("创建用户失败: %v", err)
		return nil, false, ErrUserCreateFailed
	}

	utils.LogInfo("新用户创建成功 - 用户ID: %d", userID)
	return &user, true, nil // 新用户
}

// generateDefaultNickname 生成默认昵称
func generateDefaultNickname(openid string) string {
	if len(openid) >= DefaultNicknameLength {
		return "用户" + openid[len(openid)-DefaultNicknameLength:]
	}
	return "用户" + openid
}

// CreateUserToken 创建用户token
func CreateUserToken(userID uint) (string, error) {
	utils.LogInfo("开始创建用户token - 用户ID: %d", userID)

	// 生成token
	token, err := generateToken()
	if err != nil {
		utils.LogError("生成token失败 - 用户ID: %d, 错误: %v", userID, err)
		return "", err
	}

	// 存储token
	if err := storeToken(token, userID); err != nil {
		utils.LogError("存储token失败 - 用户ID: %d, 错误: %v", userID, err)
		return "", err
	}

	utils.LogInfo("Token创建成功 - 用户ID: %d, Token长度: %d", userID, len(token))
	return token, nil
}

// generateToken 生成token
func generateToken() (string, error) {
	token, err := utils.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("生成token失败: %w", err)
	}
	return token, nil
}

// storeToken 存储token
func storeToken(token string, userID uint) error {
	expiration := utils.DefaultTokenExpiration

	utils.LogInfo("存储token到Redis - 用户ID: %d, 过期时间: %v", userID, expiration)

	// 存储token到Redis
	if err := utils.SetToken(token, userID, expiration); err != nil {
		utils.LogError("存储token到Redis失败 - 用户ID: %d, 错误: %v", userID, err)
		return ErrTokenStoreFailed
	}

	// 添加到用户token集合
	if err := utils.AddUserToken(userID, token, expiration); err != nil {
		utils.LogError("添加用户token到集合失败 - 用户ID: %d, 错误: %v", userID, err)
		// 如果添加失败，删除已创建的token
		if deleteErr := utils.DeleteToken(token); deleteErr != nil {
			utils.LogWarn("⚠️ 清理已创建的token失败 - 用户ID: %d, 错误: %v", userID, deleteErr)
		}
		return ErrTokenStoreFailed
	}

	return nil
}

// ValidateToken 验证token
func ValidateToken(token string) (*models.User, error) {
	// 脱敏记录：不记录完整的token
	utils.LogInfo("验证token - Token长度: %d", len(token))

	// 验证输入参数
	if err := validateToken(token); err != nil {
		utils.LogError("Token验证失败: %v", err)
		return nil, err
	}

	// 从Redis获取用户ID
	userID, err := utils.GetToken(token)
	if err != nil {
		utils.LogWarn("Token不存在或已过期 - Token长度: %d", len(token))
		return nil, ErrTokenNotFound
	}

	// 从数据库获取用户信息
	user, err := getUserByID(userID)
	if err != nil {
		utils.LogError("获取用户信息失败 - 用户ID: %d, 错误: %v", userID, err)
		return nil, err
	}

	utils.LogInfo("Token验证成功 - 用户ID: %d", userID)
	return user, nil
}

// validateToken 验证token格式
func validateToken(token string) error {
	if token == "" {
		return errors.New("token不能为空")
	}
	return nil
}

// getUserByID 根据ID获取用户
func getUserByID(userID uint) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// DeleteToken 删除token
func DeleteToken(token string) error {
	utils.LogInfo("删除token - Token长度: %d", len(token))

	// 验证输入参数
	if err := validateToken(token); err != nil {
		return err
	}

	// 获取用户ID
	userID, err := utils.GetToken(token)
	if err != nil {
		utils.LogWarn("Token不存在，无需删除 - Token长度: %d", len(token))
		return nil // token不存在，无需删除
	}

	// 删除token
	if err := utils.DeleteToken(token); err != nil {
		utils.LogWarn("⚠️ 删除token失败 - 用户ID: %d, 错误: %v", userID, err)
	}

	// 从用户token集合中删除
	if err := utils.RemoveUserToken(userID, token); err != nil {
		utils.LogWarn("⚠️ 从用户token集合删除失败 - 用户ID: %d, 错误: %v", userID, err)
	}

	utils.LogInfo("Token删除成功 - 用户ID: %d", userID)
	return nil
}
