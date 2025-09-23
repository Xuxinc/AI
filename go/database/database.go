package database

import (
	"ai-celebrity-simulator/config"
	"ai-celebrity-simulator/models"
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var RDB *redis.Client

// 数据库连接池配置常量
const (
	MaxIdleConns     = 10
	MaxOpenConns     = 100
	ConnMaxLifetime  = time.Hour
	PingTimeout      = 5 * time.Second
	MigrationTimeout = 30 * time.Second
)

// Redis连接池配置常量
const (
	RedisPoolSize     = 10
	RedisMinIdleConns = 5
	RedisMaxRetries   = 3
	RedisDialTimeout  = 5 * time.Second
	RedisReadTimeout  = 3 * time.Second
	RedisWriteTimeout = 3 * time.Second
)

// 错误定义
var (
	ErrConfigIncomplete = errors.New("数据库配置不完整")
	ErrDBConnection     = errors.New("数据库连接失败")
	ErrDBMigration      = errors.New("数据库迁移失败")
	ErrRedisConfig      = errors.New("redis配置不完整")
	ErrRedisConnection  = errors.New("redis连接失败")
)

// LogFunc 日志函数类型定义
type LogFunc func(format string, args ...interface{})
type LogFatalFunc func(format string, args ...interface{})
type LogWarnFunc func(format string, args ...interface{})
type LogErrorFunc func(format string, args ...interface{})

// InitDB 初始化MySQL数据库
func InitDB(logInfo LogFunc, logError LogErrorFunc) error {
	var err error

	logInfo("🔧 开始初始化MySQL数据库连接...")

	// 优先使用DSN连接字符串，如果没有则使用分项配置
	var dsn string
	var safeConnInfo string

	if config.AppConfig.MYSQLDSN != "" {
		dsn = config.AppConfig.MYSQLDSN
		safeConnInfo = "使用DSN连接字符串"
		logInfo("📡 数据库连接信息: %s", safeConnInfo)
	} else {
		// 检查分项配置是否完整
		if config.AppConfig.DBUser == "" || config.AppConfig.DBPassword == "" || config.AppConfig.DBHost == "" || config.AppConfig.DBPort == "" || config.AppConfig.DBName == "" {
			logError("❌ 数据库配置不完整，请设置MYSQL_DSN或完整的数据库分项配置")
			return ErrConfigIncomplete
		}
		// 构建MySQL DSN
		dsn = config.AppConfig.DBUser + ":" + config.AppConfig.DBPassword + "@tcp(" + config.AppConfig.DBHost + ":" + config.AppConfig.DBPort + ")/" + config.AppConfig.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
		safeConnInfo = config.AppConfig.DBUser + "@" + config.AppConfig.DBHost + ":" + config.AppConfig.DBPort + "/" + config.AppConfig.DBName
		logInfo("📡 数据库连接信息: %s", safeConnInfo)
	}

	// 配置GORM
	gormConfig := &gorm.Config{
		// 减少日志输出，只记录错误
		Logger: logger.Default.LogMode(logger.Error),
	}

	// 记录连接开始时间
	startTime := time.Now()
	DB, err = gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		logError("❌ 数据库连接失败 - 错误: %v, 连接信息: %s", err, safeConnInfo)
		return ErrDBConnection
	}

	// 记录连接耗时
	connectionTime := time.Since(startTime)
	logInfo("⚡ 数据库连接成功 - 耗时: %v", connectionTime)

	// 获取底层SQL DB以进行连接池配置
	sqlDB, err := DB.DB()
	if err != nil {
		logError("❌ 获取SQL DB实例失败: %v", err)
		return err
	}

	// 配置连接池
	sqlDB.SetMaxIdleConns(MaxIdleConns)
	sqlDB.SetMaxOpenConns(MaxOpenConns)
	sqlDB.SetConnMaxLifetime(ConnMaxLifetime)
	logInfo("🔧 数据库连接池配置完成 - 最大空闲: %d, 最大打开: %d, 生存时间: %v", MaxIdleConns, MaxOpenConns, ConnMaxLifetime)

	// 自动迁移表结构
	logInfo("🔄 开始数据库迁移...")
	migrationStart := time.Now()

	// 设置迁移超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), MigrationTimeout)
	defer cancel()

	err = DB.WithContext(ctx).AutoMigrate(&models.User{}, &models.Character{}, &models.Dialog{}, &models.Message{})
	if err != nil {
		logError("❌ 数据库迁移失败: %v", err)
		return ErrDBMigration
	}

	migrationTime := time.Since(migrationStart)
	logInfo("✅ 数据库迁移完成 - 耗时: %v", migrationTime)

	// 验证数据库连接
	pingCtx, pingCancel := context.WithTimeout(context.Background(), PingTimeout)
	defer pingCancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		logError("❌ 数据库连接验证失败: %v", err)
		return err
	}

	logInfo("✅ 数据库连接验证成功")
	logInfo("🎉 MySQL数据库初始化完成")
	return nil
}

// InitRedis 初始化Redis
func InitRedis(logInfo LogFunc, logError LogErrorFunc, logWarn LogWarnFunc) error {
	logInfo("🔧 开始初始化Redis连接...")

	// 优先使用Redis地址，如果没有则使用分项配置
	var redisAddr string

	if config.AppConfig.REDISADDR != "" {
		redisAddr = config.AppConfig.REDISADDR
		logInfo("📡 Redis连接信息: %s", redisAddr)
	} else {
		// 检查分项配置是否完整
		if config.AppConfig.RedisHost == "" || config.AppConfig.RedisPort == "" {
			logError("❌ Redis配置不完整，请设置REDIS_ADDR或完整的Redis分项配置")
			return ErrRedisConfig
		}
		// 构建Redis地址
		redisAddr = config.AppConfig.RedisHost + ":" + config.AppConfig.RedisPort
		logInfo("📡 Redis连接信息: %s", redisAddr)
	}

	// 记录连接开始时间
	startTime := time.Now()

	RDB = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     config.AppConfig.RedisPassword,
		DB:           config.AppConfig.RedisDB,
		PoolSize:     RedisPoolSize,
		MinIdleConns: RedisMinIdleConns,
		MaxRetries:   RedisMaxRetries,
		DialTimeout:  RedisDialTimeout,
		ReadTimeout:  RedisReadTimeout,
		WriteTimeout: RedisWriteTimeout,
	})

	// 测试Redis连接
	ctx, cancel := context.WithTimeout(context.Background(), PingTimeout)
	defer cancel()

	pong, err := RDB.Ping(ctx).Result()
	if err != nil {
		logError("❌ Redis连接失败 - 地址: %s, 错误: %v", redisAddr, err)
		logWarn("⚠️ Redis服务不可用，缓存功能将降级")
		return ErrRedisConnection
	}

	connectionTime := time.Since(startTime)
	logInfo("✅ Redis连接成功 - 响应: %s, 耗时: %v", pong, connectionTime)

	return nil
}
