package config

import (
	"os"

	"github.com/joho/godotenv"
)

// HTTP协议相关常量
const (
	HTTPSProtocol = "https://"
	HTTPProtocol  = "http://"
)

type Config struct {
	// 数据库配置
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	MYSQLDSN   string // 直接使用DSN连接字符串

	// Redis配置
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	REDISADDR     string // 直接使用Redis地址

	// 微信配置
	WeChatAppID     string
	WeChatAppSecret string

	// 大模型API配置
	BailianAPIKey  string // 文本模型API密钥
	BailianAPIKey2 string // 视觉模型API密钥
	BailianBaseURL string

	// OSS配置
	OSSAccessKeyID     string
	OSSAccessKeySecret string
	OSSBucket          string
	OSSEndpoint        string

	// 服务器配置
	ServerPort string
	GinMode    string // Gin运行模式：debug/release
}

var AppConfig Config

// getGinModeWithDefault 获取Gin模式，如果未设置则返回release
func getGinModeWithDefault() string {
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		return mode
	}
	return "release" // 默认为release模式
}

// LoadConfigWithLogger 使用logrus记录日志的配置加载函数
// 注意：这个函数需要在logrus初始化之后调用
func LoadConfigWithLogger(logFunc func(string, ...interface{}), errorFunc func(string, ...interface{})) {
	// 检查.env文件是否存在
	envFile := ".env"
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		errorFunc("❌ 配置文件 %s 不存在，程序无法启动", envFile)
		os.Exit(1)
	}

	// 加载.env文件
	if err := godotenv.Load(); err != nil {
		errorFunc("❌ 加载配置文件失败: %v", err)
		os.Exit(1)
	}

	AppConfig = Config{
		// 数据库配置 - 支持DSN和分项配置
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		MYSQLDSN:   os.Getenv("MYSQL_DSN"),

		// Redis配置 - 支持地址和分项配置
		RedisHost:     os.Getenv("REDIS_HOST"),
		RedisPort:     os.Getenv("REDIS_PORT"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       0,
		REDISADDR:     os.Getenv("REDIS_ADDR"),

		// 微信配置
		WeChatAppID:     os.Getenv("WECHAT_APP_ID"),
		WeChatAppSecret: os.Getenv("WECHAT_APP_SECRET"),

		// 大模型API配置
		BailianAPIKey:  os.Getenv("BAILIAN_API_KEY"),
		BailianAPIKey2: os.Getenv("BAILIAN_API_KEY_2"),
		BailianBaseURL: os.Getenv("BAILIAN_BASE_URL"),

		// OSS配置
		OSSAccessKeyID:     os.Getenv("OSS_ACCESS_KEY_ID"),
		OSSAccessKeySecret: os.Getenv("OSS_ACCESS_KEY_SECRET"),
		OSSBucket:          os.Getenv("OSS_BUCKET"),
		OSSEndpoint:        os.Getenv("OSS_ENDPOINT"),

		// 服务器配置
		ServerPort: os.Getenv("SERVER_PORT"),
		GinMode:    getGinModeWithDefault(),
	}

	// 验证关键配置，如果为空则终止程序
	validateRequiredConfigWithLogger(errorFunc)

}

// validateRequiredConfigWithLogger 使用logrus记录错误的配置验证函数
func validateRequiredConfigWithLogger(errorFunc func(string, ...interface{})) {
	var missingConfigs []string

	// 验证服务器端口
	if AppConfig.ServerPort == "" {
		missingConfigs = append(missingConfigs, "SERVER_PORT")
	}

	// 验证数据库配置
	if AppConfig.MYSQLDSN == "" {
		// 如果没有DSN，检查分项配置
		if AppConfig.DBHost == "" || AppConfig.DBPort == "" ||
			AppConfig.DBUser == "" || AppConfig.DBPassword == "" || AppConfig.DBName == "" {
			missingConfigs = append(missingConfigs, "数据库配置(MYSQL_DSN 或 DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME)")
		}
	}

	// 验证Redis配置
	if AppConfig.REDISADDR == "" {
		// 如果没有Redis地址，检查分项配置
		if AppConfig.RedisHost == "" || AppConfig.RedisPort == "" {
			missingConfigs = append(missingConfigs, "Redis配置(REDIS_ADDR 或 REDIS_HOST/REDIS_PORT)")
		}
	}

	// 验证微信配置
	if AppConfig.WeChatAppID == "" || AppConfig.WeChatAppSecret == "" {
		missingConfigs = append(missingConfigs, "微信配置(WECHAT_APP_ID/WECHAT_APP_SECRET)")
	}

	// 验证大模型API配置
	if AppConfig.BailianAPIKey == "" {
		missingConfigs = append(missingConfigs, "BAILIAN_API_KEY")
	}

	if AppConfig.BailianBaseURL == "" {
		missingConfigs = append(missingConfigs, "BAILIAN_BASE_URL")
	}

	// 如果有缺失的配置，记录错误并终止程序
	if len(missingConfigs) > 0 {
		errorFunc("❌ 缺少必需的配置项:")
		for _, config := range missingConfigs {
			errorFunc("  - %s", config)
		}
		errorFunc("请检查 .env 文件并确保所有必需配置项都已设置")
		os.Exit(1)
	}
}
