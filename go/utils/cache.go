package utils

import (
	"ai-celebrity-simulator/database"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// 缓存操作超时常量
const (
	CacheOperationTimeout = 3 * time.Second
	CacheBatchTimeout     = 10 * time.Second
	CachePatternTimeout   = 15 * time.Second
)

// 缓存配置常量
const (
	// CachePrefix 缓存键前缀
	CachePrefix = "cache:"

	// PublicCharactersExpiration 不同类型数据的缓存时间
	PublicCharactersExpiration = 1 * time.Hour    // 公开角色列表
	CharacterDetailExpiration  = 30 * time.Minute // 角色详情
	UserInfoExpiration         = 15 * time.Minute // 用户信息
	CharactersExpiration       = 10 * time.Minute // 角色列表
	SearchExpiration           = 5 * time.Minute  // 搜索结果
	ConversationsExpiration    = 5 * time.Minute  // 会话列表

	// MaxBatchKeysCount 批量操作限制
	MaxBatchKeysCount = 1000        // 单次批量操作最大键数量
	MaxCacheValueSize = 1024 * 1024 // 单个缓存值最大大小 (1MB)
)

// 缓存键模式常量
const (
	CharactersPattern = "characters:*"
	SearchPattern     = "search:*"
)

// 错误定义
var (
	ErrCacheTimeout     = errors.New("缓存操作超时")
	ErrCacheNotFound    = errors.New("缓存未找到")
	ErrCacheValueTooBig = errors.New("缓存值过大")
	ErrCacheBatchLimit  = errors.New("批量操作超出限制")
	ErrCacheKeyInvalid  = errors.New("缓存键无效")
)

// CacheService 缓存服务
type CacheService struct{}

// NewCacheService 创建缓存服务实例
func NewCacheService() *CacheService {
	return &CacheService{}
}

// Set 设置缓存（带超时和大小检查）
func (cs *CacheService) Set(key string, value interface{}, expiration time.Duration) error {
	if key == "" {
		return ErrCacheKeyInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), CacheOperationTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("%s%s", CachePrefix, key)

	// 序列化数据
	data, err := json.Marshal(value)
	if err != nil {
		LogError("缓存数据序列化失败 - 键: %s, 错误: %v", key, err)
		return fmt.Errorf("序列化缓存数据失败: %w", err)
	}

	// 检查数据大小
	if len(data) > MaxCacheValueSize {
		LogError("缓存值过大 - 键: %s, 大小: %d bytes, 限制: %d bytes", key, len(data), MaxCacheValueSize)
		return ErrCacheValueTooBig
	}

	LogDebug("设置缓存 - 键: %s, 数据大小: %d bytes, 过期时间: %v", key, len(data), expiration)

	// 存储到Redis
	err = database.RDB.Set(ctx, cacheKey, data, expiration).Err()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("设置缓存超时 - 键: %s", key)
			return fmt.Errorf("%w: 设置缓存", ErrCacheTimeout)
		}
		LogError("缓存设置失败 - 键: %s, 错误: %v", key, err)
		return fmt.Errorf("设置缓存失败: %w", err)
	}

	LogDebug("缓存设置成功 - 键: %s", key)
	return nil
}

// Get 获取缓存（带超时）
func (cs *CacheService) Get(key string, dest interface{}) error {
	if key == "" {
		return ErrCacheKeyInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), CacheOperationTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("%s%s", CachePrefix, key)

	LogDebug("获取缓存 - 键: %s", key)

	// 从Redis获取数据
	data, err := database.RDB.Get(ctx, cacheKey).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("获取缓存超时 - 键: %s", key)
			return fmt.Errorf("%w: 获取缓存", ErrCacheTimeout)
		}
		if errors.Is(err, redis.Nil) {
			LogDebug("缓存未命中 - 键: %s", key)
			return ErrCacheNotFound
		}
		LogError("获取缓存失败 - 键: %s, 错误: %v", key, err)
		return fmt.Errorf("获取缓存失败: %w", err)
	}

	// 反序列化数据
	err = json.Unmarshal([]byte(data), dest)
	if err != nil {
		LogError("缓存数据反序列化失败 - 键: %s, 错误: %v", key, err)
		return fmt.Errorf("反序列化缓存数据失败: %w", err)
	}

	LogDebug("缓存命中 - 键: %s, 数据大小: %d bytes", key, len(data))
	return nil
}

// Delete 删除缓存（带超时）
func (cs *CacheService) Delete(key string) error {
	if key == "" {
		return ErrCacheKeyInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), CacheOperationTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("%s%s", CachePrefix, key)

	LogDebug("删除缓存 - 键: %s", key)

	result, err := database.RDB.Del(ctx, cacheKey).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("删除缓存超时 - 键: %s", key)
			return fmt.Errorf("%w: 删除缓存", ErrCacheTimeout)
		}
		LogError("缓存删除失败 - 键: %s, 错误: %v", key, err)
		return fmt.Errorf("删除缓存失败: %w", err)
	}

	if result == 0 {
		LogDebug("缓存不存在，无需删除 - 键: %s", key)
	} else {
		LogDebug("缓存删除成功 - 键: %s", key)
	}

	return nil
}

// Exists 检查缓存是否存在（带超时）
func (cs *CacheService) Exists(key string) (bool, error) {
	if key == "" {
		return false, ErrCacheKeyInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), CacheOperationTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("%s%s", CachePrefix, key)

	LogDebug("检查缓存存在性 - 键: %s", key)

	result, err := database.RDB.Exists(ctx, cacheKey).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("检查缓存存在性超时 - 键: %s", key)
			return false, fmt.Errorf("%w: 检查缓存存在性", ErrCacheTimeout)
		}
		LogError("检查缓存存在性失败 - 键: %s, 错误: %v", key, err)
		return false, fmt.Errorf("检查缓存存在性失败: %w", err)
	}

	exists := result > 0
	LogDebug("缓存存在性检查完成 - 键: %s, 存在: %v", key, exists)
	return exists, nil
}

// ClearPattern 清除匹配模式的缓存（带超时和批量限制）
func (cs *CacheService) ClearPattern(pattern string) error {
	if pattern == "" {
		return ErrCacheKeyInvalid
	}

	ctx, cancel := context.WithTimeout(context.Background(), CachePatternTimeout)
	defer cancel()

	cachePattern := fmt.Sprintf("%s%s", CachePrefix, pattern)

	LogDebug("清除匹配模式的缓存 - 模式: %s", pattern)

	// 获取匹配的键
	keys, err := database.RDB.Keys(ctx, cachePattern).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("获取匹配缓存键超时 - 模式: %s", pattern)
			return fmt.Errorf("%w: 获取匹配缓存键", ErrCacheTimeout)
		}
		LogError("获取匹配的缓存键失败 - 模式: %s, 错误: %v", pattern, err)
		return fmt.Errorf("获取匹配的缓存键失败: %w", err)
	}

	if len(keys) == 0 {
		LogDebug("没有找到匹配的缓存键 - 模式: %s", pattern)
		return nil
	}

	// 检查批量操作限制
	if len(keys) > MaxBatchKeysCount {
		LogError("批量删除键数量超出限制 - 模式: %s, 键数量: %d, 限制: %d", pattern, len(keys), MaxBatchKeysCount)
		return ErrCacheBatchLimit
	}

	// 分批删除，避免单次操作过大
	batchSize := 100
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batchKeys := keys[i:end]
		if len(batchKeys) > 0 {
			batchCtx, batchCancel := context.WithTimeout(context.Background(), CacheBatchTimeout)
			err = database.RDB.Del(batchCtx, batchKeys...).Err()
			batchCancel()

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					LogError("批量删除缓存超时 - 模式: %s, 批次: %d-%d", pattern, i, end)
					return fmt.Errorf("%w: 批量删除缓存", ErrCacheTimeout)
				}
				LogError("批量删除缓存失败 - 模式: %s, 批次: %d-%d, 错误: %v", pattern, i, end, err)
				return fmt.Errorf("批量删除缓存失败: %w", err)
			}

			LogDebug("批量删除缓存成功 - 模式: %s, 批次: %d-%d", pattern, i, end)
		}
	}

	LogInfo("批量删除缓存完成 - 模式: %s, 删除键数量: %d", pattern, len(keys))
	return nil
}

// SetWithTTL 设置缓存并返回剩余TTL
func (cs *CacheService) SetWithTTL(key string, value interface{}, expiration time.Duration) (time.Duration, error) {
	if err := cs.Set(key, value, expiration); err != nil {
		return 0, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), CacheOperationTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("%s%s", CachePrefix, key)
	ttl, err := database.RDB.TTL(ctx, cacheKey).Result()
	if err != nil {
		LogWarn("获取缓存TTL失败 - 键: %s, 错误: %v", key, err)
		return expiration, nil // 返回设置的过期时间
	}

	return ttl, nil
}

// BatchGet 批量获取缓存
func (cs *CacheService) BatchGet(keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	if len(keys) > MaxBatchKeysCount {
		return nil, ErrCacheBatchLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), CacheBatchTimeout)
	defer cancel()

	// 构建完整的缓存键
	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = fmt.Sprintf("%s%s", CachePrefix, key)
	}

	LogDebug("批量获取缓存 - 键数量: %d", len(keys))

	// 批量获取
	results, err := database.RDB.MGet(ctx, cacheKeys...).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("批量获取缓存超时 - 键数量: %d", len(keys))
			return nil, fmt.Errorf("%w: 批量获取缓存", ErrCacheTimeout)
		}
		LogError("批量获取缓存失败 - 键数量: %d, 错误: %v", len(keys), err)
		return nil, fmt.Errorf("批量获取缓存失败: %w", err)
	}

	// 解析结果
	resultMap := make(map[string]interface{})
	hitCount := 0

	for i, result := range results {
		if result != nil {
			var value interface{}
			if err := json.Unmarshal([]byte(result.(string)), &value); err != nil {
				LogWarn("缓存数据反序列化失败 - 键: %s, 错误: %v", keys[i], err)
				continue
			}
			resultMap[keys[i]] = value
			hitCount++
		}
	}

	LogDebug("批量获取缓存完成 - 请求: %d, 命中: %d", len(keys), hitCount)
	return resultMap, nil
}

// 缓存键生成函数

// GetPublicCharactersKey 获取公开角色列表缓存键
func GetPublicCharactersKey() string {
	return "public_characters"
}

// GetCharacterDetailKey 获取角色详情缓存键
func GetCharacterDetailKey(characterID uint) string {
	return fmt.Sprintf("character_detail:%d", characterID)
}

// GetUserInfoKey 获取用户信息缓存键
func GetUserInfoKey(userID uint) string {
	return fmt.Sprintf("user_info:%d", userID)
}

// GetCharactersKey 获取角色列表缓存键
func GetCharactersKey(userID uint) string {
	return fmt.Sprintf("characters:%d", userID)
}

// GetSearchKey 获取搜索结果缓存键
func GetSearchKey(query string) string {
	return fmt.Sprintf("search:%s", query)
}

// GetConversationsKey 获取会话列表缓存键
func GetConversationsKey(userID uint) string {
	return fmt.Sprintf("conversations:%d", userID)
}

// 缓存清理函数

// ClearUserCaches 清除用户相关缓存（带错误处理）
func ClearUserCaches(userID uint) error {
	cacheSvc := NewCacheService()

	LogInfo("开始清除用户缓存 - 用户ID: %d", userID)

	var errorList []error

	// 清除用户信息缓存
	if err := cacheSvc.Delete(GetUserInfoKey(userID)); err != nil && !errors.Is(err, ErrCacheNotFound) {
		LogWarn("清除用户信息缓存失败 - 用户ID: %d, 错误: %v", userID, err)
		errorList = append(errorList, fmt.Errorf("清除用户信息缓存失败: %w", err))
	}

	// 清除用户角色列表缓存
	if err := cacheSvc.Delete(GetCharactersKey(userID)); err != nil && !errors.Is(err, ErrCacheNotFound) {
		LogWarn("清除用户角色列表缓存失败 - 用户ID: %d, 错误: %v", userID, err)
		errorList = append(errorList, fmt.Errorf("清除用户角色列表缓存失败: %w", err))
	}

	// 清除用户会话列表缓存
	if err := cacheSvc.Delete(GetConversationsKey(userID)); err != nil && !errors.Is(err, ErrCacheNotFound) {
		LogWarn("清除用户会话列表缓存失败 - 用户ID: %d, 错误: %v", userID, err)
		errorList = append(errorList, fmt.Errorf("清除用户会话列表缓存失败: %w", err))
	}

	if len(errorList) > 0 {
		LogError("用户缓存清除部分失败 - 用户ID: %d, 错误数量: %d", userID, len(errorList))
		return fmt.Errorf("用户缓存清除部分失败: %d个错误", len(errorList))
	}

	LogInfo("用户缓存清除完成 - 用户ID: %d", userID)
	return nil
}

// ClearCharacterCaches 清除角色相关缓存（带错误处理）
func ClearCharacterCaches(characterID uint) error {
	cacheSvc := NewCacheService()

	LogInfo("开始清除角色缓存 - 角色ID: %d", characterID)

	var errorList []error

	// 清除角色详情缓存
	if err := cacheSvc.Delete(GetCharacterDetailKey(characterID)); err != nil && !errors.Is(err, ErrCacheNotFound) {
		LogWarn("清除角色详情缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
		errorList = append(errorList, fmt.Errorf("清除角色详情缓存失败: %w", err))
	}

	// 清除公开角色列表缓存
	if err := cacheSvc.Delete(GetPublicCharactersKey()); err != nil && !errors.Is(err, ErrCacheNotFound) {
		LogWarn("清除公开角色列表缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
		errorList = append(errorList, fmt.Errorf("清除公开角色列表缓存失败: %w", err))
	}

	// 清除所有用户的角色列表缓存
	if err := cacheSvc.ClearPattern(CharactersPattern); err != nil {
		LogWarn("清除用户角色列表缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
		errorList = append(errorList, fmt.Errorf("清除用户角色列表缓存失败: %w", err))
	}

	// 清除搜索缓存
	if err := cacheSvc.ClearPattern(SearchPattern); err != nil {
		LogWarn("清除搜索缓存失败 - 角色ID: %d, 错误: %v", characterID, err)
		errorList = append(errorList, fmt.Errorf("清除搜索缓存失败: %w", err))
	}

	if len(errorList) > 0 {
		LogError("角色缓存清除部分失败 - 角色ID: %d, 错误数量: %d", characterID, len(errorList))
		return fmt.Errorf("角色缓存清除部分失败: %d个错误", len(errorList))
	}

	LogInfo("角色缓存清除完成 - 角色ID: %d", characterID)
	return nil
}

// ClearConversationCaches 清除会话相关缓存（带错误处理）
func ClearConversationCaches(userID uint) error {
	cacheSvc := NewCacheService()

	LogInfo("开始清除会话缓存 - 用户ID: %d", userID)

	// 清除用户会话列表缓存
	if err := cacheSvc.Delete(GetConversationsKey(userID)); err != nil && !errors.Is(err, ErrCacheNotFound) {
		LogError("清除会话缓存失败 - 用户ID: %d, 错误: %v", userID, err)
		return fmt.Errorf("清除会话缓存失败: %w", err)
	}

	LogInfo("会话缓存清除完成 - 用户ID: %d", userID)
	return nil
}
