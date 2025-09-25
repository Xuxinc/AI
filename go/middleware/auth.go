package middleware

import (
	"ai-celebrity-simulator/services"
	"ai-celebrity-simulator/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// StatusUnauthorized HTTP状态码常量
const (
	StatusUnauthorized = http.StatusUnauthorized
)

// CodeUnauthorized 业务状态码常量
const (
	CodeUnauthorized = 401
)

// 响应消息常量
const (
	MessageMissingToken     = "缺少认证token"
	MessageTokenFormatError = "token格式错误"
	MessageTokenInvalid     = "token无效或已过期"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.LogWarn("认证失败 - 缺少Authorization头 - IP: %s, User-Agent: %s",
				c.ClientIP(), c.GetHeader("User-Agent"))
			c.JSON(StatusUnauthorized, gin.H{
				"code":    CodeUnauthorized,
				"message": MessageMissingToken,
			})
			c.Abort()
			return
		}

		// 解析token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			utils.LogWarn("认证失败 - token格式错误 - IP: %s, User-Agent: %s, Header: %s",
				c.ClientIP(), c.GetHeader("User-Agent"), authHeader)
			c.JSON(StatusUnauthorized, gin.H{
				"code":    CodeUnauthorized,
				"message": MessageTokenFormatError,
			})
			c.Abort()
			return
		}

		// 验证token
		user, err := services.ValidateToken(token)
		if err != nil {
			utils.LogError("认证失败 - token验证失败 - IP: %s, User-Agent: %s, Token长度: %d, 错误: %v",
				c.ClientIP(), c.GetHeader("User-Agent"), len(token), err)
			c.JSON(StatusUnauthorized, gin.H{
				"code":    CodeUnauthorized,
				"message": MessageTokenInvalid,
			})
			c.Abort()
			return
		}

		// 认证成功，记录日志
		utils.LogInfo("认证成功 - 用户ID: %d, IP: %s, User-Agent: %s",
			user.ID, c.ClientIP(), c.GetHeader("User-Agent"))

		// 将用户信息存储到上下文中
		c.Set("user_id", user.ID)
		c.Set("user", user)

		c.Next()
	}
}
