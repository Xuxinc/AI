package routes

import (
	"ai-celebrity-simulator/handlers"
	"ai-celebrity-simulator/middleware"
	"ai-celebrity-simulator/websocket"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// 公开路由
	api := r.Group("/api")
	{
		// 用户相关
		api.POST("/user/login", handlers.UserLogin)
		api.POST("/user/wechat-info", middleware.AuthMiddleware(), handlers.GetWeChatUserInfoByCode)
		api.GET("/user/info", middleware.AuthMiddleware(), handlers.GetUserInfo)
		api.PUT("/user/info", middleware.AuthMiddleware(), handlers.UpdateUserInfo)
		api.POST("/user/upload-avatar", middleware.AuthMiddleware(), handlers.UploadUserAvatar) // 新增
		api.POST("/user/logout", middleware.AuthMiddleware(), handlers.UserLogout)

		// 角色相关
		api.GET("/characters/public", handlers.GetPublicCharacters)
		api.GET("/characters", middleware.AuthMiddleware(), handlers.GetCharacters)
		api.GET("/characters/search", middleware.AuthMiddleware(), handlers.SearchCharacter)
		api.POST("/characters/generate-celebrity", middleware.AuthMiddleware(), handlers.GenerateCelebrityCharacter)
		api.POST("/characters/generate-custom", middleware.AuthMiddleware(), handlers.GenerateCustomCharacter)
		api.POST("/characters/upload-avatar", middleware.AuthMiddleware(), handlers.UploadCustomCharacterAvatar)
		api.GET("/characters/:id", middleware.AuthMiddleware(), handlers.GetCharacterDetail)
		api.PUT("/characters/:id", middleware.AuthMiddleware(), handlers.UpdateCharacter)
		api.DELETE("/characters/:id", middleware.AuthMiddleware(), handlers.DeleteCharacter)      // 新增：删除角色
		api.POST("/characters/:id/public", middleware.AuthMiddleware(), handlers.PublicCharacter) // 新增：公开角色

		// 音色相关
		api.POST("/upload/voice", middleware.AuthMiddleware(), handlers.UploadVoiceAudio)
		api.POST("/voice/create", middleware.AuthMiddleware(), handlers.CreateVoiceModel)

		// 聊天相关
		api.POST("/chat/message", middleware.AuthMiddleware(), handlers.SendChatMessage)
		api.GET("/chat/dialog", middleware.AuthMiddleware(), handlers.GetDialog)
		api.POST("/chat/upload-images", middleware.AuthMiddleware(), handlers.UploadImages)   // 新增：图片上传
		api.POST("/chat/delete-message", middleware.AuthMiddleware(), handlers.DeleteMessage) // 新增：删除消息

		// 会话相关
		api.GET("/conversations", middleware.AuthMiddleware(), handlers.GetUserConversations)
		api.POST("/conversations/pin", middleware.AuthMiddleware(), handlers.PinConversation)
		api.DELETE("/conversations/:characterId", middleware.AuthMiddleware(), handlers.DeleteConversation)

		// 消息相关
		api.POST("/messages", middleware.AuthMiddleware(), handlers.SaveCallDurationMessage)
	}

	// WebSocket路由
	ws := r.Group("/ws")
	{
		ws.GET("/voice-chat", websocket.HandleVoiceChat)
	}
}
