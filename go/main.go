package main

import (
	"ai-celebrity-simulator/config"
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/routes"
	"ai-celebrity-simulator/services"
	"ai-celebrity-simulator/utils"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	// 初始化logrus日志系统
	logConfig := utils.LogConfig{
		LogDir:   "logs",
		AppName:  "ai-celebrity-simulator",
		Level:    logrus.InfoLevel,
		MaxAge:   7 * 24 * time.Hour, // 保留7天
		Rotation: time.Hour,          // 每小时轮转
	}

	if err := utils.InitLogrus(logConfig); err != nil {
		log.Fatalf("Logrus日志系统初始化失败: %v", err)
	}

	utils.LogInfo("🚀 AI名人模拟器服务启动中...")

	// 加载配置（使用logrus记录日志）
	config.LoadConfigWithLogger(utils.LogInfo, utils.LogError)

	// 初始化数据库连接
	if err := database.InitDB(utils.LogInfo, utils.LogError); err != nil {
		utils.LogFatal("❌ 数据库初始化失败: %v", err)
	}
	utils.LogInfo("💾 数据库连接初始化完成")

	// 初始化Redis连接
	if err := database.InitRedis(utils.LogInfo, utils.LogError, utils.LogWarn); err != nil {
		utils.LogWarn("⚠️ Redis初始化失败: %v", err)
		utils.LogWarn("🔴 Redis缓存功能将不可用")
	} else {
		utils.LogInfo("🔴 Redis连接初始化完成")
	}

	// 初始化OSS服务
	if err := services.InitOSSService(); err != nil {
		utils.LogWarn("⚠️ OSS服务初始化失败: %v", err)
		utils.LogWarn("📸 图片上传功能将不可用")
	} else {
		utils.LogInfo("☁️ OSS服务初始化成功")
	}

	// 设置Gin模式（通过配置控制，与日志系统保持一致）
	gin.SetMode(config.AppConfig.GinMode)
	utils.LogInfo("🔧 Gin模式设置为: %s", config.AppConfig.GinMode)

	r := gin.New()

	// 使用基于logrus的中间件
	r.Use(utils.LogrusErrorRecoveryMiddleware())
	r.Use(utils.LogrusRequestLoggerMiddleware())

	// 配置CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})
	utils.LogInfo("🔧 CORS配置完成")

	// 设置路由
	routes.SetupRoutes(r)
	utils.LogInfo("🛣️ 路由设置完成")

	// 设置优雅关闭
	setupGracefulShutdown()

	// 启动HTTP服务器（nginx会处理HTTPS）
	if config.AppConfig.ServerPort == "" {
		utils.LogFatal("❌ 服务器启动失败: 未配置SERVER_PORT环境变量")
	}
	port := ":" + config.AppConfig.ServerPort
	utils.LogInfo("🌐 HTTP服务器启动在端口 %s ", port)
	if err := r.Run(port); err != nil {
		utils.LogFatal("❌ HTTP服务器启动失败: %v", err)
	}
}

// setupGracefulShutdown 设置优雅关闭
func setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		utils.LogInfo("🛑 接收到关闭信号，正在优雅关闭服务...")

		utils.LogInfo("✅ 服务已优雅关闭")
		os.Exit(0)
	}()
}
