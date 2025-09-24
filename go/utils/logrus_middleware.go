package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// 中间件配置常量
const (
	// HTTPStatusErrorThreshold HTTP状态码阈值
	HTTPStatusErrorThreshold   = 500 // 服务器错误阈值
	HTTPStatusWarningThreshold = 400 // 客户端错误阈值

	// StatusInternalServerError HTTP状态码常量
	StatusInternalServerError = http.StatusInternalServerError

	// InternalServerErrorMessage 响应消息
	InternalServerErrorMessage = "服务器内部错误"

	// LogFieldMethod 日志字段名
	LogFieldMethod    = "method"
	LogFieldPath      = "path"
	LogFieldClientIP  = "client_ip"
	LogFieldStatus    = "status"
	LogFieldLatency   = "latency"
	LogFieldUserAgent = "user_agent"
	LogFieldError     = "error"
	LogFieldPanic     = "panic"

	// LogMessageHTTPComplete 日志消息
	LogMessageHTTPComplete   = "🌐 HTTP请求处理完成"
	LogMessagePanicRecovered = "💥 系统异常捕获"
)

// LogrusRequestLoggerMiddleware 基于logrus的HTTP请求日志中间件
func LogrusRequestLoggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// 计算请求处理时间
		latency := param.Latency

		// 创建日志字段
		fields := logrus.Fields{
			LogFieldMethod:    param.Method,
			LogFieldPath:      param.Path,
			LogFieldClientIP:  param.ClientIP,
			LogFieldStatus:    param.StatusCode,
			LogFieldLatency:   latency.String(),
			LogFieldUserAgent: param.Request.UserAgent(),
		}

		// 根据状态码选择日志级别
		if param.StatusCode >= HTTPStatusErrorThreshold {
			if param.ErrorMessage != "" {
				fields[LogFieldError] = param.ErrorMessage
			}
			LogWithFields(fields).Error(LogMessageHTTPComplete)
		} else if param.StatusCode >= HTTPStatusWarningThreshold {
			LogWithFields(fields).Warn(LogMessageHTTPComplete)
		} else {
			LogWithFields(fields).Info(LogMessageHTTPComplete)
		}

		return ""
	})
}

// LogrusErrorRecoveryMiddleware 基于logrus的错误恢复中间件
func LogrusErrorRecoveryMiddleware() gin.HandlerFunc {
	return gin.RecoveryWithWriter(gin.DefaultWriter, func(c *gin.Context, recovered interface{}) {
		panicFields := logrus.Fields{
			LogFieldMethod:   c.Request.Method,
			LogFieldPath:     c.Request.URL.Path,
			LogFieldClientIP: c.ClientIP(),
			LogFieldPanic:    recovered,
		}

		LogWithFields(panicFields).Error(LogMessagePanicRecovered)

		// 返回标准化的错误响应
		c.JSON(StatusInternalServerError, gin.H{
			"code":    StatusInternalServerError,
			"message": InternalServerErrorMessage,
		})
	})
}
