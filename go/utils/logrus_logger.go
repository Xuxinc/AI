package utils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

// 日志配置常量
const (
	// LogDirPermission 文件权限
	LogDirPermission = 0755

	// MaskPrefix 敏感信息脱敏配置
	MaskPrefix           = 2
	MaskSuffix           = 2
	MaskMinLength        = 4
	PhoneMaskLength      = 11
	PhoneMaskKeepPrefix  = 3
	PhoneMaskKeepSuffix  = 4
	IDCardMaskLength     = 18
	IDCardMaskKeepPrefix = 6
	IDCardMaskKeepSuffix = 4
	EmailMaskKeepPrefix  = 2
	BankCardMaskMinLen   = 8
	BankCardMaskKeepLen  = 4

	// LogRotationFormat 日志轮转文件名格式
	LogRotationFormat = ".%Y%m%d%H"

	// DefaultAppVersion 默认应用版本
	DefaultAppVersion = "1.0.0"

	// EnvGinMode 环境变量
	EnvGinMode = "GIN_MODE"
	GinRelease = "release"

	// LogLevelFormat 日志级别字符串格式
	LogLevelFormat = `"level":"%s"`

	// MaskChar 脱敏字符
	MaskChar = "*"
	MaskText = "***"
)

// 错误定义
var (
	ErrLogDirCreateFailed     = errors.New("创建日志目录失败")
	ErrLogRotatorCreateFailed = errors.New("创建日志轮转器失败")
	ErrLoggerInitFailed       = errors.New("日志系统初始化失败")
)

var (
	// GlobalLogger 全局logrus实例
	GlobalLogger *logrus.Logger
	// 敏感信息脱敏正则表达式
	sensitivePatterns = map[string]*regexp.Regexp{
		"password": regexp.MustCompile(`(?i)(password|pwd|passwd|pass)\s*[:=]\s*["']?([^"'\s,}]+)["']?`),
		"token":    regexp.MustCompile(`(?i)(token|jwt|auth|bearer)\s*[:=]\s*["']?([^"'\s,}]+)["']?`),
		"apikey":   regexp.MustCompile(`(?i)(api[_-]?key|apikey|access[_-]?key)\s*[:=]\s*["']?([^"'\s,}]+)["']?`),
		"secret":   regexp.MustCompile(`(?i)(secret|private[_-]?key)\s*[:=]\s*["']?([^"'\s,}]+)["']?`),
		"phone":    regexp.MustCompile(`1[3-9]\d{9}`),
		"idcard":   regexp.MustCompile(`\d{17}[\dXx]`),
		"email":    regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		"bankcard": regexp.MustCompile(`\d{16,19}`),
	}
)

// LogConfig 日志配置
type LogConfig struct {
	LogDir   string        // 日志目录
	AppName  string        // 应用名称
	Level    logrus.Level  // 日志级别
	MaxAge   time.Duration // 日志保留时间
	Rotation time.Duration // 轮转间隔
}

// InitLogrus 初始化logrus日志系统
func InitLogrus(config LogConfig) error {
	// 验证配置
	if err := validateLogConfig(config); err != nil {
		return fmt.Errorf("%w: %v", ErrLoggerInitFailed, err)
	}

	// 确保日志目录存在
	if err := createLogDirectory(config.LogDir); err != nil {
		return fmt.Errorf("%w: %v", ErrLogDirCreateFailed, err)
	}

	// 创建logrus实例
	GlobalLogger = logrus.New()
	GlobalLogger.SetLevel(config.Level)

	// 创建多个writer
	var writers []io.Writer

	// 1. 控制台输出（开发环境）
	if shouldLogToConsole() {
		consoleWriter := createConsoleWriter(config.Level)
		if consoleWriter != nil {
			writers = append(writers, os.Stdout)
		}
	}

	// 2. 设置JSON格式化器用于文件输出
	GlobalLogger.SetFormatter(createJSONFormatter())

	// 3. 通用日志文件（所有级别）
	allLogWriter, err := createAllLogWriter(config)
	if err != nil {
		return fmt.Errorf("%w: 通用日志文件: %v", ErrLogRotatorCreateFailed, err)
	}
	writers = append(writers, NewSensitiveFilter(allLogWriter))

	// 4. 错误日志文件（仅ERROR和FATAL级别）
	errorLogWriter, err := createErrorLogWriter(config)
	if err != nil {
		return fmt.Errorf("%w: 错误日志文件: %v", ErrLogRotatorCreateFailed, err)
	}

	// 创建错误级别过滤器
	errorFilterWriter := &LevelFilterWriter{
		Writer: NewSensitiveFilter(errorLogWriter),
		Levels: []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
	}
	writers = append(writers, errorFilterWriter)

	// 设置多重输出
	GlobalLogger.SetOutput(io.MultiWriter(writers...))

	// 添加调用者信息
	GlobalLogger.SetReportCaller(true)

	// 记录初始化成功日志
	logInitializationSuccess(config)

	return nil
}

// validateLogConfig 验证日志配置
func validateLogConfig(config LogConfig) error {
	if config.LogDir == "" {
		return errors.New("日志目录不能为空")
	}
	if config.AppName == "" {
		return errors.New("应用名称不能为空")
	}
	if config.MaxAge <= 0 {
		return errors.New("日志保留时间必须大于0")
	}
	if config.Rotation <= 0 {
		return errors.New("轮转间隔必须大于0")
	}
	return nil
}

// createLogDirectory 创建日志目录
func createLogDirectory(logDir string) error {
	return os.MkdirAll(logDir, LogDirPermission)
}

// shouldLogToConsole 判断是否应该输出到控制台
func shouldLogToConsole() bool {
	return os.Getenv(EnvGinMode) != GinRelease
}

// createConsoleWriter 创建控制台输出writer
func createConsoleWriter(level logrus.Level) *logrus.Logger {
	consoleFormatter := &logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
	}
	// 为控制台创建单独的logger，避免formatter冲突
	consoleLogger := logrus.New()
	consoleLogger.SetFormatter(consoleFormatter)
	consoleLogger.SetLevel(level)
	consoleLogger.SetOutput(os.Stdout)
	return consoleLogger
}

// createJSONFormatter 创建JSON格式化器
func createJSONFormatter() *logrus.JSONFormatter {
	return &logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyFunc:  "caller",
		},
	}
}

// createAllLogWriter 创建通用日志文件writer
func createAllLogWriter(config LogConfig) (io.Writer, error) {
	allLogPath := filepath.Join(config.LogDir, fmt.Sprintf("%s_all.log", config.AppName))
	allLogPathPattern := filepath.Join(config.LogDir, config.AppName+"_all"+LogRotationFormat+".log")
	return rotatelogs.New(
		allLogPathPattern,
		rotatelogs.WithLinkName(allLogPath),
		rotatelogs.WithMaxAge(config.MaxAge),
		rotatelogs.WithRotationTime(config.Rotation),
	)
}

// createErrorLogWriter 创建错误日志文件writer
func createErrorLogWriter(config LogConfig) (io.Writer, error) {
	errorLogPath := filepath.Join(config.LogDir, fmt.Sprintf("%s_error.log", config.AppName))
	errorLogPathPattern := filepath.Join(config.LogDir, config.AppName+"_error"+LogRotationFormat+".log")
	return rotatelogs.New(
		errorLogPathPattern,
		rotatelogs.WithLinkName(errorLogPath),
		rotatelogs.WithMaxAge(config.MaxAge),
		rotatelogs.WithRotationTime(config.Rotation),
	)
}

// logInitializationSuccess 记录初始化成功日志
func logInitializationSuccess(config LogConfig) {
	GlobalLogger.WithFields(logrus.Fields{
		"app":     config.AppName,
		"version": DefaultAppVersion,
	}).Info("📝 Logrus日志系统初始化成功")
}

// SensitiveFilter 敏感信息过滤器
type SensitiveFilter struct {
	writer io.Writer
}

// NewSensitiveFilter 创建敏感信息过滤器
func NewSensitiveFilter(writer io.Writer) *SensitiveFilter {
	return &SensitiveFilter{writer: writer}
}

// Write 实现io.Writer接口，对敏感信息进行脱敏
func (sf *SensitiveFilter) Write(p []byte) (n int, err error) {
	content := string(p)

	// 对敏感信息进行脱敏
	for name, pattern := range sensitivePatterns {
		content = sf.maskSensitiveData(content, name, pattern)
	}

	return sf.writer.Write([]byte(content))
}

// maskSensitiveData 脱敏敏感数据
func (sf *SensitiveFilter) maskSensitiveData(content, name string, pattern *regexp.Regexp) string {
	switch name {
	case "password", "token", "apikey", "secret":
		return sf.maskKeyValuePairs(content, pattern)
	case "phone":
		return sf.maskPhone(content, pattern)
	case "idcard":
		return sf.maskIDCard(content, pattern)
	case "email":
		return sf.maskEmail(content, pattern)
	case "bankcard":
		return sf.maskBankCard(content, pattern)
	default:
		return content
	}
}

// maskKeyValuePairs 脱敏键值对
func (sf *SensitiveFilter) maskKeyValuePairs(content string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := pattern.FindStringSubmatch(match)
		if len(parts) >= 3 {
			key := parts[1]
			value := parts[2]
			maskedValue := sf.maskString(value)
			return fmt.Sprintf("%s: %s", key, maskedValue)
		}
		return match
	})
}

// maskPhone 脱敏手机号
func (sf *SensitiveFilter) maskPhone(content string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		if len(match) == PhoneMaskLength {
			return match[:PhoneMaskKeepPrefix] + strings.Repeat(MaskChar, PhoneMaskLength-PhoneMaskKeepPrefix-PhoneMaskKeepSuffix) + match[PhoneMaskLength-PhoneMaskKeepSuffix:]
		}
		return match
	})
}

// maskIDCard 脱敏身份证
func (sf *SensitiveFilter) maskIDCard(content string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		if len(match) == IDCardMaskLength {
			return match[:IDCardMaskKeepPrefix] + strings.Repeat(MaskChar, IDCardMaskLength-IDCardMaskKeepPrefix-IDCardMaskKeepSuffix) + match[IDCardMaskLength-IDCardMaskKeepSuffix:]
		}
		return match
	})
}

// maskEmail 脱敏邮箱
func (sf *SensitiveFilter) maskEmail(content string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := strings.Split(match, "@")
		if len(parts) == 2 && len(parts[0]) > EmailMaskKeepPrefix {
			return parts[0][:EmailMaskKeepPrefix] + MaskText + "@" + parts[1]
		}
		return match
	})
}

// maskBankCard 脱敏银行卡
func (sf *SensitiveFilter) maskBankCard(content string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		if len(match) >= BankCardMaskMinLen {
			return match[:BankCardMaskKeepLen] + strings.Repeat(MaskChar, len(match)-BankCardMaskKeepLen*2) + match[len(match)-BankCardMaskKeepLen:]
		}
		return match
	})
}

// maskString 通用字符串脱敏
func (sf *SensitiveFilter) maskString(value string) string {
	if len(value) > MaskMinLength {
		return value[:MaskPrefix] + strings.Repeat(MaskChar, len(value)-MaskPrefix-MaskSuffix) + value[len(value)-MaskSuffix:]
	}
	return strings.Repeat(MaskChar, len(value))
}

// LevelFilterWriter 日志级别过滤器
type LevelFilterWriter struct {
	Writer io.Writer
	Levels []logrus.Level
}

// Write 实现io.Writer接口，只写入指定级别的日志
func (lfw *LevelFilterWriter) Write(p []byte) (n int, err error) {
	// 这里需要解析日志级别，简化处理：检查日志内容中的级别标识
	content := string(p)

	for _, level := range lfw.Levels {
		levelStr := fmt.Sprintf(LogLevelFormat, level.String())
		if strings.Contains(content, levelStr) {
			return lfw.Writer.Write(p)
		}
	}

	// 如果不是指定级别，返回写入长度但不实际写入
	return len(p), nil
}

// LogDebug 全局日志方法
func LogDebug(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Debugf(format, args...)
	}
}

func LogInfo(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Infof(format, args...)
	}
}

func LogWarn(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Warnf(format, args...)
	}
}

func LogError(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Errorf(format, args...)
	}
}

func LogFatal(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.Fatalf(format, args...)
	}
}

// LogWithFields 添加字段
func LogWithFields(fields map[string]interface{}) *logrus.Entry {
	if GlobalLogger != nil {
		return GlobalLogger.WithFields(fields)
	}
	return logrus.NewEntry(logrus.New())
}
