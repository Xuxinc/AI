package websocket

import (
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/services"
	"ai-celebrity-simulator/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// WebSocket配置常量
const (
	// ReadTimeout 连接超时配置
	ReadTimeout  = 60 * time.Second
	WriteTimeout = 10 * time.Second
	PingPeriod   = 54 * time.Second
	PongWait     = 60 * time.Second

	// ASRStartTimeout ASR服务超时配置
	ASRStartTimeout = 10 * time.Second
	TTSTaskDelay    = 500 * time.Millisecond

	// ResultChannelBuffer 缓冲区大小
	ResultChannelBuffer  = 100
	StartedChannelBuffer = 1

	// StatusUnauthorized HTTP状态码
	StatusUnauthorized = http.StatusUnauthorized

	// CodeUnauthorized 业务状态码
	CodeUnauthorized = 401

	// MessageMissingToken 响应消息
	MessageMissingToken       = "缺少认证token"
	MessageInvalidToken       = "token无效或已过期"
	MessageCallStarted        = "通话已开始"
	MessageInterruptConfirmed = "已停止当前语音合成"

	// MessageTypeCallStarted WebSocket消息类型
	MessageTypeCallStarted        = "call_started"
	MessageTypeRecognitionResult  = "recognition_result"
	MessageTypeAIResponse         = "ai_response"
	MessageTypeStreamAudio        = "stream_audio"
	MessageTypeError              = "error"
	MessageTypePong               = "pong"
	MessageTypeInterruptConfirmed = "interrupt_confirmed"

	// ControlTypeStartCall 控制消息类型
	ControlTypeStartCall         = "start_call"
	ControlTypeEndCall           = "end_call"
	ControlTypePing              = "ping"
	ControlTypeInterruptPlayback = "interrupt_playback"

	// DBValueYes 数据库字段值
	DBValueYes = "yes"
	DBRoleUser = "user"
	DBRoleAI   = "ai"

	// BearerPrefix Bearer前缀
	BearerPrefix = "Bearer "

	// ActionPattern1 正则表达式模式
	ActionPattern1    = `\*[^*]*\*`
	ActionPattern2    = `（[^）]*）`
	WhitespacePattern = `\s+`
	SentencePattern   = `[。！？!?]+`
)

// 错误定义
var (
	ErrCharacterNotFound   = errors.New("角色不存在")
	ErrDialogQueryFailed   = errors.New("查找会话失败")
	ErrDialogCreateFailed  = errors.New("新建会话失败")
	ErrASRInitFailed       = errors.New("语音识别服务初始化失败")
	ErrTTSInitFailed       = errors.New("语音合成服务初始化失败")
	ErrCharacterInfoFailed = errors.New("获取角色信息失败")
	ErrMessageSaveFailed   = errors.New("保存消息失败")
	ErrUnknownMessageType  = errors.New("未知的消息类型")
)

// VoiceChatConnection 语音聊天WebSocket连接
type VoiceChatConnection struct {
	conn              *websocket.Conn
	userID            uint
	characterID       uint
	characterName     string
	dialogID          uint
	isActive          bool
	recognitionSvc    *services.VoiceRecognitionService
	frameIndex        int
	isProcessing      bool                       // 是否正在处理AI回复
	currentTTSService *services.StreamTTSService // 当前TTS服务
	ttsMutex          sync.Mutex                 // TTS服务互斥锁
	clientIP          string                     // 客户端IP
	ctx               context.Context            // 连接上下文
	cancel            context.CancelFunc         // 取消函数
}

// VoiceChatMessage 语音聊天消息结构
type VoiceChatMessage struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

// VoiceChatRequest 语音聊天请求结构
type VoiceChatRequest struct {
	Type        string `json:"type"`
	CharacterID uint   `json:"character_id"`
	AudioData   []byte `json:"audio_data,omitempty"`
}

// voiceUpgrader WebSocket升级器配置
var voiceUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境需要更严格的检查
	},
}

// HandleVoiceChat 处理语音聊天WebSocket连接
func HandleVoiceChat(c *gin.Context) {
	utils.LogInfo("收到语音聊天WebSocket连接请求 - 客户端IP: %s", c.ClientIP())

	// 从URL参数或请求头获取token
	token := extractToken(c)
	if token == "" {
		utils.LogWarn("语音聊天连接失败: 缺少认证token - 客户端IP: %s", c.ClientIP())
		c.JSON(StatusUnauthorized, gin.H{
			"code":    CodeUnauthorized,
			"message": MessageMissingToken,
		})
		return
	}

	// 验证token
	user, err := validateUserToken(token, c.ClientIP())
	if err != nil {
		utils.LogWarn("语音聊天连接失败: %v - 客户端IP: %s", err, c.ClientIP())
		c.JSON(StatusUnauthorized, gin.H{
			"code":    CodeUnauthorized,
			"message": MessageInvalidToken,
		})
		return
	}

	utils.LogInfo("用户认证成功 - 用户ID: %d, 客户端IP: %s", user.ID, c.ClientIP())

	// 升级HTTP连接为WebSocket
	conn, err := upgradeToWebSocket(c)
	if err != nil {
		utils.LogError("升级WebSocket连接失败 - 用户ID: %d, 错误: %v", user.ID, err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			utils.LogWarn("⚠️ 关闭WebSocket连接失败: %v", err)
		}
	}()

	utils.LogInfo("WebSocket连接升级成功 - 用户ID: %d, 客户端IP: %s", user.ID, c.ClientIP())

	// 创建语音聊天连接
	voiceConn := createVoiceChatConnection(conn, user.ID, c.ClientIP())

	// 处理语音聊天
	voiceConn.handleVoiceChat()
}

// extractToken 提取认证token
func extractToken(c *gin.Context) string {
	token := c.Query("token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			token = strings.TrimPrefix(authHeader, BearerPrefix)
		}
	}
	return token
}

// validateUserToken 验证用户token
func validateUserToken(token, clientIP string) (*models.User, error) {
	utils.LogDebug("验证用户token - Token长度: %d, 客户端IP: %s", len(token), clientIP)

	user, err := services.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("token无效或已过期: %w", err)
	}

	return user, nil
}

// upgradeToWebSocket 升级到WebSocket连接
func upgradeToWebSocket(c *gin.Context) (*websocket.Conn, error) {
	conn, err := voiceUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket升级失败: %w", err)
	}

	// 设置WebSocket超时配置
	if err := conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		utils.LogWarn("⚠️ 设置读取超时失败: %v", err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}
	conn.SetPongHandler(func(string) error {
		if err := conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
			utils.LogWarn("⚠️ 设置Pong读取超时失败: %v", err)
		}
		return nil
	})

	return conn, nil
}

// createVoiceChatConnection 创建语音聊天连接
func createVoiceChatConnection(conn *websocket.Conn, userID uint, clientIP string) *VoiceChatConnection {
	ctx, cancel := context.WithCancel(context.Background())

	return &VoiceChatConnection{
		conn:     conn,
		userID:   userID,
		isActive: true,
		clientIP: clientIP,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// handleVoiceChat 处理语音聊天
func (vc *VoiceChatConnection) handleVoiceChat() {
	defer func() {
		vc.cleanup()
	}()

	// 初始化语音识别服务
	if err := vc.initializeASRService(); err != nil {
		utils.LogError("初始化语音识别服务失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("语音识别服务初始化失败")
		return
	}

	// 发送连接成功消息
	vc.sendConnectionSuccess()

	// 启动语音识别结果处理
	if err := vc.startRecognitionProcessing(); err != nil {
		utils.LogError("启动语音识别处理失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		return
	}

	utils.LogInfo("语音聊天WebSocket连接成功 - 用户ID: %d", vc.userID)

	// 启动心跳检测
	go vc.startHeartbeat()

	// 主循环：处理客户端消息
	vc.processClientMessages()
}

// cleanup 清理资源
func (vc *VoiceChatConnection) cleanup() {
	vc.isActive = false

	// 取消上下文
	if vc.cancel != nil {
		vc.cancel()
	}

	// 关闭ASR服务
	if vc.recognitionSvc != nil {
		if err := vc.recognitionSvc.Close(); err != nil {
			utils.LogWarn("⚠️ 关闭ASR服务失败: %v", err)
		}
	}

	// 停止当前TTS服务
	vc.interruptCurrentTTS()

	utils.LogInfo("语音聊天连接资源清理完成 - 用户ID: %d", vc.userID)
}

// initializeASRService 初始化ASR服务
func (vc *VoiceChatConnection) initializeASRService() error {
	recognitionSvc, err := services.NewVoiceRecognitionService()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrASRInitFailed, err)
	}

	vc.recognitionSvc = recognitionSvc
	return nil
}

// sendConnectionSuccess 发送连接成功消息
func (vc *VoiceChatConnection) sendConnectionSuccess() {
	vc.sendMessage(MessageTypeCallStarted, map[string]interface{}{
		"message":   MessageCallStarted,
		"dialog_id": vc.dialogID,
	})
}

// startRecognitionProcessing 启动语音识别处理
func (vc *VoiceChatConnection) startRecognitionProcessing() error {
	// 创建识别结果通道和启动通道
	resultChan := make(chan string, ResultChannelBuffer)
	startedChan := make(chan struct{}, StartedChannelBuffer)

	// 启动识别结果监听
	go vc.recognitionSvc.ListenForResults(resultChan, startedChan)

	// 处理识别结果
	go vc.handleRecognitionResults(resultChan)

	// 等待ASR任务启动
	go vc.waitForASRStartup(startedChan)

	return nil
}

// waitForASRStartup 等待ASR服务启动
func (vc *VoiceChatConnection) waitForASRStartup(startedChan <-chan struct{}) {
	select {
	case <-startedChan:
		utils.LogInfo("ASR任务已启动，准备接收PCM音频数据 - 用户ID: %d", vc.userID)
	case <-time.After(ASRStartTimeout):
		utils.LogError("等待ASR任务启动超时 - 用户ID: %d", vc.userID)
		vc.sendError("ASR服务启动超时")
	case <-vc.ctx.Done():
		utils.LogDebug("ASR启动等待被取消 - 用户ID: %d", vc.userID)
	}
}

// startHeartbeat 启动心跳检测
func (vc *VoiceChatConnection) startHeartbeat() {
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !vc.isActive {
				return
			}

			if err := vc.conn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
				utils.LogWarn("⚠️ 设置心跳写入超时失败: %v", err)
			}
			if err := vc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				utils.LogError("发送心跳失败 - 用户ID: %d, 错误: %v", vc.userID, err)
				return
			}

		case <-vc.ctx.Done():
			return
		}
	}
}

// processClientMessages 处理客户端消息
func (vc *VoiceChatConnection) processClientMessages() {
	for vc.isActive {
		// 设置读取超时
		if err := vc.conn.SetReadDeadline(time.Now().Add(ReadTimeout)); err != nil {
			utils.LogWarn("⚠️ 设置读取超时失败: %v", err)
		}

		_, message, err := vc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				utils.LogError("WebSocket连接异常关闭 - 用户ID: %d, 错误: %v", vc.userID, err)
			} else {
				utils.LogDebug("WebSocket连接正常关闭 - 用户ID: %d", vc.userID)
			}
			break
		}

		// 处理消息
		if err := vc.handleMessage(message); err != nil {
			utils.LogError("处理消息失败 - 用户ID: %d, 错误: %v", vc.userID, err)
			vc.sendError("消息处理失败")
		}
	}
}

// handleMessage 处理客户端消息
func (vc *VoiceChatConnection) handleMessage(message []byte) error {
	// 尝试解析为JSON消息
	var chatMsg VoiceChatRequest
	if err := json.Unmarshal(message, &chatMsg); err == nil {
		// 处理控制消息
		return vc.handleControlMessage(chatMsg)
	}

	// 如果不是JSON，则认为是音频数据
	return vc.handleAudioData(message)
}

// handleControlMessage 处理控制消息
func (vc *VoiceChatConnection) handleControlMessage(msg VoiceChatRequest) error {
	utils.LogDebug("收到控制消息 - 用户ID: %d, 类型: %s", vc.userID, msg.Type)

	switch msg.Type {
	case ControlTypeStartCall:
		return vc.handleStartCall(msg.CharacterID)

	case ControlTypeEndCall:
		return vc.handleEndCall()

	case ControlTypePing:
		return vc.handlePing()

	case ControlTypeInterruptPlayback:
		return vc.handleInterruptPlayback()

	default:
		utils.LogWarn("未知的控制消息类型 - 用户ID: %d, 类型: %s", vc.userID, msg.Type)
		return fmt.Errorf("%w: %s", ErrUnknownMessageType, msg.Type)
	}
}

// handleStartCall 处理开始通话
func (vc *VoiceChatConnection) handleStartCall(characterID uint) error {
	vc.characterID = characterID

	if err := vc.initializeCall(); err != nil {
		utils.LogError("初始化通话失败 - 用户ID: %d, 角色ID: %d, 错误: %v",
			vc.userID, characterID, err)
		return err
	}

	vc.sendMessage(MessageTypeCallStarted, MessageCallStarted)
	utils.LogInfo("通话开始成功 - 用户ID: %d, 角色ID: %d, 会话ID: %d",
		vc.userID, vc.characterID, vc.dialogID)

	return nil
}

// handleEndCall 处理结束通话
func (vc *VoiceChatConnection) handleEndCall() error {
	utils.LogInfo("收到结束通话指令 - 用户ID: %d", vc.userID)

	// 发送finish-task指令给ASR
	if vc.recognitionSvc != nil {
		if err := vc.recognitionSvc.FinishTask(); err != nil {
			utils.LogError("发送ASR finish-task指令失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		} else {
			utils.LogDebug("已发送ASR finish-task指令 - 用户ID: %d", vc.userID)
		}
	}

	// 关闭WebSocket连接
	if err := vc.conn.Close(); err != nil {
		utils.LogWarn("⚠️ 关闭WebSocket连接失败: %v", err)
	}
	return nil
}

// handlePing 处理心跳
func (vc *VoiceChatConnection) handlePing() error {
	vc.sendMessage(MessageTypePong, "")
	return nil
}

// handleInterruptPlayback 处理打断播放
func (vc *VoiceChatConnection) handleInterruptPlayback() error {
	utils.LogInfo("收到用户打断音频播放信号 - 用户ID: %d", vc.userID)
	vc.interruptCurrentTTS()
	vc.sendMessage(MessageTypeInterruptConfirmed, MessageInterruptConfirmed)
	return nil
}

// handleAudioData 处理音频数据
func (vc *VoiceChatConnection) handleAudioData(audioData []byte) error {
	// 检查音频数据是否为空
	if len(audioData) == 0 {
		utils.LogDebug("收到空音频数据，跳过处理 - 用户ID: %d", vc.userID)
		return nil
	}

	// 递增帧序号
	vc.frameIndex++

	// 直接将音频数据发送给ASR
	if vc.recognitionSvc != nil {
		if err := vc.recognitionSvc.SendAudioData(audioData); err != nil {
			utils.LogError("发送音频数据到ASR失败 - 用户ID: %d, 帧序号: %d, 错误: %v",
				vc.userID, vc.frameIndex, err)
			return fmt.Errorf("发送音频数据失败: %w", err)
		}

		utils.LogDebug("音频数据发送成功 - 用户ID: %d, 帧序号: %d, 数据大小: %d bytes",
			vc.userID, vc.frameIndex, len(audioData))
	} else {
		utils.LogError("语音识别服务未初始化 - 用户ID: %d", vc.userID)
		return ErrASRInitFailed
	}

	return nil
}

// handleRecognitionResults 处理语音识别结果
func (vc *VoiceChatConnection) handleRecognitionResults(resultChan <-chan string) {
	for {
		select {
		case resultMessage, ok := <-resultChan:
			if !ok {
				utils.LogDebug("识别结果通道已关闭 - 用户ID: %d", vc.userID)
				return
			}

			if resultMessage == "" {
				continue
			}

			vc.processRecognitionResult(resultMessage)

		case <-vc.ctx.Done():
			utils.LogDebug("识别结果处理被取消 - 用户ID: %d", vc.userID)
			return
		}
	}
}

// processRecognitionResult 处理单个识别结果
func (vc *VoiceChatConnection) processRecognitionResult(resultMessage string) {
	// 解析结果消息
	parts := strings.Split(resultMessage, ":")
	if len(parts) < 4 {
		utils.LogWarn("识别结果格式错误 - 用户ID: %d, 消息: %s", vc.userID, resultMessage)
		return
	}

	isFinalStr := parts[1]
	text := strings.Join(parts[3:], ":") // 处理文本中可能包含冒号的情况
	isFinal := isFinalStr == "true"

	utils.LogInfo("语音识别结果 - 用户ID: %d, 文本长度: %d, 是否最终: %t",
		vc.userID, len(text), isFinal)

	// 发送识别结果给前端（包括中间结果和最终结果）
	vc.sendMessage(MessageTypeRecognitionResult, map[string]interface{}{
		"text":     text,
		"is_final": isFinal,
	})

	// 只有最终结果才保存到数据库并调用AI回复
	if isFinal {
		vc.processFinalRecognitionResult(text)
	}
}

// processFinalRecognitionResult 处理最终识别结果
func (vc *VoiceChatConnection) processFinalRecognitionResult(text string) {
	// 防止重复处理
	if vc.isProcessing {
		utils.LogWarn("正在处理AI回复，跳过重复请求 - 用户ID: %d", vc.userID)
		return
	}

	// 设置处理状态
	vc.isProcessing = true

	// 保存用户语音消息
	if err := vc.saveUserVoiceMessage(text); err != nil {
		utils.LogError("保存用户语音消息失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.isProcessing = false
		return
	}

	// 清除会话列表缓存
	if err := utils.ClearConversationCaches(vc.userID); err != nil {
		utils.LogWarn("清除会话缓存失败 - 用户ID: %d, 错误: %v", vc.userID, err)
	}

	// 异步生成AI回复
	go vc.generateAIResponseAsync(text)
}

// saveUserVoiceMessage 保存用户语音消息
func (vc *VoiceChatConnection) saveUserVoiceMessage(text string) error {
	voiceMessage := models.Message{
		DialogID:   vc.dialogID,
		Content:    text,
		PictureURL: "",
		IsVoice:    DBValueYes,
		Role:       DBRoleUser,
		Time:       time.Now(),
	}

	if err := database.DB.Create(&voiceMessage).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrMessageSaveFailed, err)
	}

	utils.LogDebug("用户语音消息保存成功 - 用户ID: %d, 会话ID: %d, 文本长度: %d",
		vc.userID, vc.dialogID, len(text))

	return nil
}

// generateAIResponseAsync 异步生成AI回复
func (vc *VoiceChatConnection) generateAIResponseAsync(recognizedText string) {
	defer func() {
		vc.isProcessing = false // 处理完成后重置状态

		if r := recover(); r != nil {
			utils.LogError("AI回复生成发生panic - 用户ID: %d, panic: %v", vc.userID, r)
			vc.sendError("AI回复生成异常")
		}
	}()

	utils.LogInfo("开始生成AI回复 - 用户ID: %d, 文本长度: %d", vc.userID, len(recognizedText))
	vc.generateAIResponse(recognizedText)
}

// initializeCall 初始化通话
func (vc *VoiceChatConnection) initializeCall() error {
	// 获取角色信息
	character, err := vc.getCharacterInfo()
	if err != nil {
		return err
	}
	vc.characterName = character.Name

	// 获取或创建会话
	dialog, err := vc.getOrCreateDialog()
	if err != nil {
		return err
	}
	vc.dialogID = dialog.ID

	// 清除会话列表缓存
	if err := utils.ClearConversationCaches(vc.userID); err != nil {
		utils.LogWarn("清除会话缓存失败 - 用户ID: %d, 错误: %v", vc.userID, err)
	}

	return nil
}

// getCharacterInfo 获取角色信息
func (vc *VoiceChatConnection) getCharacterInfo() (*models.Character, error) {
	var character models.Character
	if err := database.DB.First(&character, vc.characterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.LogError("角色不存在 - 用户ID: %d, 角色ID: %d", vc.userID, vc.characterID)
			return nil, ErrCharacterNotFound
		}
		utils.LogError("获取角色信息失败 - 用户ID: %d, 角色ID: %d, 错误: %v",
			vc.userID, vc.characterID, err)
		return nil, fmt.Errorf("%w: %v", ErrCharacterInfoFailed, err)
	}

	utils.LogDebug("角色信息获取成功 - 用户ID: %d, 角色ID: %d, 角色名: %s",
		vc.userID, vc.characterID, character.Name)

	return &character, nil
}

// getOrCreateDialog 获取或创建会话
func (vc *VoiceChatConnection) getOrCreateDialog() (*models.Dialog, error) {
	// 查找是否已有会话
	var dialog models.Dialog
	result := database.DB.Where("user_id = ? AND character_id = ?", vc.userID, vc.characterID).First(&dialog)

	if result.Error == nil {
		// 已有会话，直接使用
		utils.LogDebug("使用现有会话 - 用户ID: %d, 角色ID: %d, 会话ID: %d",
			vc.userID, vc.characterID, dialog.ID)
		return &dialog, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		utils.LogError("查找会话失败 - 用户ID: %d, 角色ID: %d, 错误: %v",
			vc.userID, vc.characterID, result.Error)
		return nil, fmt.Errorf("%w: %v", ErrDialogQueryFailed, result.Error)
	}

	// 没有会话，新建
	dialog = models.Dialog{
		UserID:      vc.userID,
		CharacterID: vc.characterID,
		CreatedAt:   time.Now(),
	}

	if err := database.DB.Create(&dialog).Error; err != nil {
		utils.LogError("新建会话失败 - 用户ID: %d, 角色ID: %d, 错误: %v",
			vc.userID, vc.characterID, err)
		return nil, fmt.Errorf("%w: %v", ErrDialogCreateFailed, err)
	}

	utils.LogInfo("新建会话成功 - 用户ID: %d, 角色ID: %d, 会话ID: %d",
		vc.userID, vc.characterID, dialog.ID)

	return &dialog, nil
}

// generateAIResponse 生成AI回复
func (vc *VoiceChatConnection) generateAIResponse(userText string) {
	// 检查连接状态
	if !vc.isActive {
		utils.LogDebug("连接已关闭，跳过AI回复生成 - 用户ID: %d", vc.userID)
		return
	}

	startTime := time.Now()
	utils.LogInfo("开始生成AI回复 - 用户ID: %d, 文本长度: %d", vc.userID, len(userText))

	// 获取角色信息
	character, err := vc.getCharacterInfo()
	if err != nil {
		utils.LogError("生成AI回复时获取角色信息失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("获取角色信息失败")
		return
	}

	// 生成AI回复文本
	rawAIResponse, err := services.GenerateAIReplyWithHistory(vc.dialogID, character.Prompt, userText)
	if err != nil {
		utils.LogError("AI回复生成失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("AI回复生成失败")
		return
	}

	// 过滤AI回复中的动作描述
	aiResponse := vc.filterActionDescriptions(rawAIResponse)
	utils.LogInfo("AI回复生成完成 - 用户ID: %d, 原始长度: %d, 过滤后长度: %d",
		vc.userID, len(rawAIResponse), len(aiResponse))

	// 检查连接状态
	if !vc.isActive {
		utils.LogDebug("AI文本生成完成，但连接已关闭 - 用户ID: %d", vc.userID)
		return
	}

	textGenerationTime := time.Since(startTime)
	utils.LogInfo("AI文本生成完成 - 用户ID: %d, 耗时: %v", vc.userID, textGenerationTime)

	// 先发送AI文本回复给前端
	vc.sendMessage(MessageTypeAIResponse, aiResponse)

	// 异步生成语音
	go vc.generateStreamVoice(aiResponse, startTime)
}

// generateStreamVoice 流式语音生成
func (vc *VoiceChatConnection) generateStreamVoice(text string, startTime time.Time) {
	utils.LogInfo("开始流式语音合成 - 用户ID: %d, 文本长度: %d", vc.userID, len(text))

	// 获取角色信息，包括voice_model
	character, err := vc.getCharacterInfo()
	if err != nil {
		utils.LogError("生成语音时获取角色信息失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("获取角色信息失败")
		return
	}

	// 先打断之前的TTS服务（如果有）
	vc.interruptCurrentTTS()

	// 创建新的流式TTS服务
	ttsService, err := vc.createTTSService(character.VoiceModel)
	if err != nil {
		utils.LogError("创建TTS服务失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("语音合成服务初始化失败")
		return
	}

	// 处理TTS任务
	audioCount := vc.processTTSTask(ttsService, text)

	// 保存AI消息到数据库
	vc.saveAIVoiceMessage(text)

	// 清除会话列表缓存
	if err := utils.ClearConversationCaches(vc.userID); err != nil {
		utils.LogWarn("清除会话缓存失败 - 用户ID: %d, 错误: %v", vc.userID, err)
	}

	totalTime := time.Since(startTime)
	utils.LogInfo("流式语音合成完成 - 用户ID: %d, 音频片段: %d, 总耗时: %v",
		vc.userID, audioCount, totalTime)
}

// createTTSService 创建TTS服务
func (vc *VoiceChatConnection) createTTSService(voiceModel string) (*services.StreamTTSService, error) {
	ttsService, err := services.NewStreamTTSService(voiceModel)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTTSInitFailed, err)
	}

	// 保存当前TTS服务引用
	vc.ttsMutex.Lock()
	vc.currentTTSService = ttsService
	vc.ttsMutex.Unlock()

	utils.LogDebug("TTS服务创建成功 - 用户ID: %d, 音色模型: %s", vc.userID, voiceModel)
	return ttsService, nil
}

// processTTSTask 处理TTS任务
func (vc *VoiceChatConnection) processTTSTask(ttsService *services.StreamTTSService, text string) int {
	defer func() {
		// 清理TTS服务引用
		vc.ttsMutex.Lock()
		if vc.currentTTSService == ttsService {
			vc.currentTTSService = nil
		}
		vc.ttsMutex.Unlock()

		// 关闭TTS服务
		if err := ttsService.Close(); err != nil {
			utils.LogWarn("⚠️ 关闭TTS服务失败: %v", err)
		}
	}()

	// 等待task-started事件
	time.Sleep(TTSTaskDelay)

	// 发送文本进行合成
	if err := ttsService.SendText(text); err != nil {
		utils.LogError("发送TTS文本失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("语音合成失败")
		return 0
	}

	// 发送finish-task指令
	if err := ttsService.FinishTask(); err != nil {
		utils.LogError("发送TTS finish指令失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		vc.sendError("语音合成结束失败")
		return 0
	}

	// 处理音频流
	return vc.processAudioStream(ttsService)
}

// processAudioStream 处理音频流
func (vc *VoiceChatConnection) processAudioStream(ttsService *services.StreamTTSService) int {
	audioChan := ttsService.GetAudioChannel()
	audioCount := 0

	for audioData := range audioChan {
		// 检查是否已被打断
		if !vc.checkTTSContinuation(ttsService) {
			utils.LogDebug("TTS服务已被打断，停止音频转发 - 用户ID: %d", vc.userID)
			break
		}

		if !vc.isActive {
			utils.LogDebug("连接已关闭，停止音频转发 - 用户ID: %d", vc.userID)
			break
		}

		audioCount++

		// 发送音频数据给前端
		vc.sendMessage(MessageTypeStreamAudio, map[string]interface{}{
			"audio_data": audioData,
			"sequence":   audioCount,
		})

		if audioCount%50 == 0 { // 每50个片段记录一次
			utils.LogDebug("音频转发进度 - 用户ID: %d, 片段: %d", vc.userID, audioCount)
		}
	}

	return audioCount
}

// checkTTSContinuation 检查TTS是否应该继续
func (vc *VoiceChatConnection) checkTTSContinuation(ttsService *services.StreamTTSService) bool {
	vc.ttsMutex.Lock()
	defer vc.ttsMutex.Unlock()
	return vc.currentTTSService == ttsService
}

// saveAIVoiceMessage 保存AI语音消息
func (vc *VoiceChatConnection) saveAIVoiceMessage(text string) {
	aiMessage := models.Message{
		DialogID:   vc.dialogID,
		Content:    text,
		PictureURL: "",
		IsVoice:    DBValueYes,
		Role:       DBRoleAI,
		VoiceURL:   "",
		Time:       time.Now(),
	}

	if err := database.DB.Create(&aiMessage).Error; err != nil {
		utils.LogError("保存AI语音消息失败 - 用户ID: %d, 错误: %v", vc.userID, err)
	} else {
		utils.LogDebug("AI语音消息保存成功 - 用户ID: %d, 会话ID: %d", vc.userID, vc.dialogID)
	}
}

// splitBySentence 按句号切分文本
func (vc *VoiceChatConnection) splitBySentence(text string) []string {
	// 按句号、感叹号、问号切分
	sentences := regexp.MustCompile(SentencePattern).Split(text, -1)

	var result []string
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence != "" {
			result = append(result, sentence)
		}
	}

	// 如果没有切分出句子，返回原文
	if len(result) == 0 {
		result = append(result, text)
	}

	return result
}

// filterActionDescriptions 过滤AI回复中的动作描述
func (vc *VoiceChatConnection) filterActionDescriptions(text string) string {
	// 移除 *动作* 格式的内容
	re1 := regexp.MustCompile(ActionPattern1)
	filtered := re1.ReplaceAllString(text, "")

	re2 := regexp.MustCompile(ActionPattern2)
	filtered = re2.ReplaceAllString(filtered, "")

	// 清理多余的空格和换行
	re3 := regexp.MustCompile(WhitespacePattern)
	filtered = re3.ReplaceAllString(filtered, " ")

	// 去除首尾空格
	filtered = strings.TrimSpace(filtered)

	return filtered
}

// interruptCurrentTTS 打断当前TTS服务
func (vc *VoiceChatConnection) interruptCurrentTTS() {
	vc.ttsMutex.Lock()
	defer vc.ttsMutex.Unlock()

	if vc.currentTTSService != nil {
		utils.LogInfo("正在停止当前TTS服务 - 用户ID: %d", vc.userID)

		// 发送finish-task指令结束当前合成
		if err := vc.currentTTSService.FinishTask(); err != nil {
			utils.LogError("发送TTS finish-task指令失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		}

		// 关闭TTS连接
		if err := vc.currentTTSService.Close(); err != nil {
			utils.LogError("关闭TTS连接失败 - 用户ID: %d, 错误: %v", vc.userID, err)
		}

		vc.currentTTSService = nil
		utils.LogDebug("当前TTS服务已停止 - 用户ID: %d", vc.userID)
	}
}

// sendMessage 发送消息给客户端
func (vc *VoiceChatConnection) sendMessage(msgType string, data interface{}) {
	if !vc.isActive {
		utils.LogDebug("连接已关闭，无法发送消息 - 用户ID: %d, 类型: %s", vc.userID, msgType)
		return
	}

	message := VoiceChatMessage{
		Type: msgType,
		Data: data,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		utils.LogError("消息序列化失败 - 用户ID: %d, 类型: %s, 错误: %v", vc.userID, msgType, err)
		return
	}

	// 设置写入超时
	if err := vc.conn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := vc.conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
		utils.LogError("发送消息失败 - 用户ID: %d, 类型: %s, 错误: %v", vc.userID, msgType, err)
		return
	}

	utils.LogDebug("消息发送成功 - 用户ID: %d, 类型: %s", vc.userID, msgType)
}

// sendError 发送错误消息
func (vc *VoiceChatConnection) sendError(message string) {
	if !vc.isActive {
		utils.LogDebug("连接已关闭，无法发送错误消息 - 用户ID: %d, 消息: %s", vc.userID, message)
		return
	}

	utils.LogWarn("发送错误消息 - 用户ID: %d, 消息: %s", vc.userID, message)

	vc.sendMessage(MessageTypeError, map[string]string{
		"message": message,
	})
}
