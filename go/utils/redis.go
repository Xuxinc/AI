package utils

import (
	"ai-celebrity-simulator/database"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// Redis操作超时常量
const (
	RedisOperationTimeout   = 5 * time.Second
	RedisTransactionTimeout = 15 * time.Second
)

// Token相关常量
const (
	// TokenPrefix Token相关key前缀
	TokenPrefix      = "token:"
	UserTokensPrefix = "user_tokens:"

	// DefaultTokenExpiration 默认过期时间
	DefaultTokenExpiration = 30 * 24 * time.Hour // 30天
)

// 错误定义
var (
	ErrTokenNotFound    = errors.New("token不存在")
	ErrRedisTimeout     = errors.New("redis操作超时")
	ErrRedisTransaction = errors.New("redis事务执行失败")
)

// RedisOperationResult Redis操作结果
type RedisOperationResult struct {
	Success bool
	Error   error
}

// SetToken 设置token到Redis（带超时和事务）
func SetToken(token string, userID uint, expiration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), RedisOperationTimeout)
	defer cancel()

	key := fmt.Sprintf("%s%s", TokenPrefix, token)

	LogDebug("设置Token - Token长度: %d, 用户ID: %d, 过期时间: %v", len(token), userID, expiration)

	err := database.RDB.Set(ctx, key, userID, expiration).Err()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("设置Token超时 - Token长度: %d, 用户ID: %d", len(token), userID)
			return fmt.Errorf("%w: 设置Token", ErrRedisTimeout)
		}
		LogError("设置Token失败 - Token长度: %d, 用户ID: %d, 错误: %v", len(token), userID, err)
		return fmt.Errorf("设置Token失败: %w", err)
	}

	LogDebug("Token设置成功 - 用户ID: %d", userID)
	return nil
}

// GetToken 从Redis获取token对应的用户ID（带超时）
func GetToken(token string) (uint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), RedisOperationTimeout)
	defer cancel()

	key := fmt.Sprintf("%s%s", TokenPrefix, token)

	LogDebug("获取Token - Token长度: %d", len(token))

	userIDStr, err := database.RDB.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("获取Token超时 - Token长度: %d", len(token))
			return 0, fmt.Errorf("%w: 获取Token", ErrRedisTimeout)
		}
		if errors.Is(err, redis.Nil) {
			LogWarn("Token不存在 - Token长度: %d", len(token))
			return 0, ErrTokenNotFound
		}
		LogError("获取Token失败 - Token长度: %d, 错误: %v", len(token), err)
		return 0, fmt.Errorf("获取Token失败: %w", err)
	}

	var userID uint
	n, err := fmt.Sscanf(userIDStr, "%d", &userID)
	if err != nil || n != 1 {
		LogError("Token值解析失败 - Token长度: %d, 值: %s", len(token), userIDStr)
		return 0, fmt.Errorf("token值解析失败: %w", err)
	}

	LogDebug("Token获取成功 - 用户ID: %d", userID)
	return userID, nil
}

// DeleteToken 删除token（带超时）
func DeleteToken(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), RedisOperationTimeout)
	defer cancel()

	key := fmt.Sprintf("%s%s", TokenPrefix, token)

	LogDebug("删除Token - Token长度: %d", len(token))

	result, err := database.RDB.Del(ctx, key).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("删除Token超时 - Token长度: %d", len(token))
			return fmt.Errorf("%w: 删除Token", ErrRedisTimeout)
		}
		LogError("删除Token失败 - Token长度: %d, 错误: %v", len(token), err)
		return fmt.Errorf("删除Token失败: %w", err)
	}

	if result == 0 {
		LogWarn("Token不存在，无需删除 - Token长度: %d", len(token))
	} else {
		LogDebug("Token删除成功 - Token长度: %d", len(token))
	}

	return nil
}

// AddUserToken 添加用户token到集合（带事务保证一致性）
func AddUserToken(userID uint, token string, expiration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), RedisTransactionTimeout)
	defer cancel()

	userTokensKey := fmt.Sprintf("%s%d", UserTokensPrefix, userID)

	LogDebug("添加用户Token - 用户ID: %d, Token长度: %d", userID, len(token))

	// 使用事务保证一致性
	err := database.RDB.Watch(ctx, func(tx *redis.Tx) error {
		// 在事务中执行操作
		pipe := tx.TxPipeline()

		// 添加到用户token集合
		pipe.SAdd(ctx, userTokensKey, token)
		// 设置集合过期时间
		pipe.Expire(ctx, userTokensKey, expiration)

		_, err := pipe.Exec(ctx)
		return err
	}, userTokensKey)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("添加用户Token事务超时 - 用户ID: %d, Token长度: %d", userID, len(token))
			return fmt.Errorf("%w: 添加用户Token事务", ErrRedisTimeout)
		}
		LogError("添加用户Token事务失败 - 用户ID: %d, Token长度: %d, 错误: %v", userID, len(token), err)
		return fmt.Errorf("%w: %v", ErrRedisTransaction, err)
	}

	LogDebug("用户Token添加成功 - 用户ID: %d, Token长度: %d", userID, len(token))
	return nil
}

// RemoveUserToken 从用户token集合中移除（带超时）
func RemoveUserToken(userID uint, token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), RedisOperationTimeout)
	defer cancel()

	key := fmt.Sprintf("%s%d", UserTokensPrefix, userID)

	LogDebug("移除用户Token - 用户ID: %d, Token长度: %d", userID, len(token))

	result, err := database.RDB.SRem(ctx, key, token).Result()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			LogError("移除用户Token超时 - 用户ID: %d, Token长度: %d", userID, len(token))
			return fmt.Errorf("%w: 移除用户Token", ErrRedisTimeout)
		}
		LogError("移除用户Token失败 - 用户ID: %d, Token长度: %d, 错误: %v", userID, len(token), err)
		return fmt.Errorf("移除用户Token失败: %w", err)
	}

	if result == 0 {
		LogWarn("用户Token不存在，无需移除 - 用户ID: %d, Token长度: %d", userID, len(token))
	} else {
		LogDebug("用户Token移除成功 - 用户ID: %d, Token长度: %d", userID, len(token))
	}

	return nil
}
