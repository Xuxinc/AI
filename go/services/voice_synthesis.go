package services

import (
	"ai-celebrity-simulator/utils"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WebSocket配置常量
const (
	TTSDialTimeout   = 10 * time.Second
	TTSWriteTimeout  = 10 * time.Second
	TTSReadTimeout   = 60 * time.Second
	TTSChannelBuffer = 100
)

// TTS模型配置常量
const (
	TTSModel         = "cosyvoice-v2"
	TTSFunction      = "SpeechSynthesizer"
	TTSFormat        = "pcm"
	TTSSampleRate    = 22050
	TTSVolume        = 80
	TTSRate          = 1.1
	TTSPitch         = 1
	TTSStreamingMode = "duplex"
	TTSTextType      = "PlainText"
)

// DefaultVoiceModel 默认音色常量
const (
	DefaultVoiceModel = "longxiaochun_v2"
)

// 错误定义
var (
	ErrTTSAPIKeyMissing    = errors.New("TTS API密钥未配置")
	ErrTTSConnectionFailed = errors.New("TTS WebSocket连接失败")
	ErrTTSTaskCreateFailed = errors.New("TTS任务创建失败")
	ErrTTSTaskStartFailed  = errors.New("TTS任务启动失败")
	ErrTTSTextSendFailed   = errors.New("TTS文本发送失败")
	ErrTTSTaskFinishFailed = errors.New("TTS任务结束失败")
)

const (
	CosyVoiceWSURL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"
)

// StreamTTSService 流式语音合成服务
type StreamTTSService struct {
	conn        *websocket.Conn
	taskID      string
	audioChan   chan []byte
	isConnected bool
}

// TTSHeader TTS指令头部结构
type TTSHeader struct {
	Action       string                 `json:"action"`
	TaskID       string                 `json:"task_id"`
	Streaming    string                 `json:"streaming"`
	Event        string                 `json:"event,omitempty"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

// TTSParams TTS参数结构
type TTSParams struct {
	TextType   string  `json:"text_type"`
	Voice      string  `json:"voice"`
	Format     string  `json:"format"`
	SampleRate int     `json:"sample_rate"`
	Volume     int     `json:"volume"`
	Rate       float64 `json:"rate"`
	Pitch      int     `json:"pitch"`
}

// TTSInput TTS输入结构
type TTSInput struct {
	Text string `json:"text,omitempty"`
}

// TTSPayload TTS载荷结构
type TTSPayload struct {
	TaskGroup  string    `json:"task_group,omitempty"`
	Task       string    `json:"task,omitempty"`
	Function   string    `json:"function,omitempty"`
	Model      string    `json:"model,omitempty"`
	Parameters TTSParams `json:"parameters,omitempty"`
	Input      TTSInput  `json:"input"`
}

// TTSEvent TTS事件结构
type TTSEvent struct {
	Header  TTSHeader  `json:"header"`
	Payload TTSPayload `json:"payload"`
}

// NewStreamTTSService 创建流式TTS服务
func NewStreamTTSService(voiceModel string) (*StreamTTSService, error) {
	utils.LogInfo("🔊 开始创建流式TTS服务 - 音色模型: %s", voiceModel)

	startTime := time.Now()

	// 判断是否为自定义音色（格式：cosyvoice-v2-celebrity-xxxxx）
	isCustomVoice := strings.Contains(voiceModel, "cosyvoice-v2")

	// 获取并验证API密钥
	apiKey, err := getTTSAPIKey(isCustomVoice)
	if err != nil {
		utils.LogError("❌ TTS服务初始化失败: %v", err)
		return nil, err
	}

	// 建立WebSocket连接
	conn, err := connectTTSWebSocket(apiKey)
	if err != nil {
		utils.LogError("❌ CosyVoice WebSocket连接失败: %v", err)
		return nil, err
	}

	connectionTime := time.Since(startTime)
	utils.LogInfo("✅ CosyVoice WebSocket连接建立成功 - 耗时: %v", connectionTime)

	// 生成任务ID
	taskID := generateTTSTaskID()
	utils.LogInfo("🆔 生成TTS任务ID: %s", taskID)

	// 创建音频通道
	audioChan := make(chan []byte, TTSChannelBuffer)

	service := &StreamTTSService{
		conn:        conn,
		taskID:      taskID,
		audioChan:   audioChan,
		isConnected: true,
	}

	// 发送run-task指令
	if err := service.sendRunTaskCmd(voiceModel); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭WebSocket连接失败: %v", closeErr)
		}
		utils.LogError("❌ 发送run-task指令失败: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrTTSTaskCreateFailed, err)
	}

	// 启动音频接收协程
	go service.receiveAudio()
	utils.LogInfo("🎧 音频接收协程已启动")

	utils.LogInfo("🎉 流式TTS服务创建成功")
	return service, nil
}

// getTTSAPIKey 获取TTS API密钥
func getTTSAPIKey(isCustomVoice bool) (string, error) {
	var apiKey string

	if isCustomVoice {
		// 自定义音色使用第二个API Key
		apiKey = os.Getenv("BAILIAN_API_KEY_2")
		if apiKey == "" {
			return "", fmt.Errorf("%w: 未配置BAILIAN_API_KEY_2", ErrTTSAPIKeyMissing)
		}
		utils.LogInfo("🎨 使用自定义音色API密钥 - 长度: %d", len(apiKey))
	} else {
		// 系统预设音色使用第一个API Key
		apiKey = os.Getenv("BAILIAN_API_KEY")
		if apiKey == "" {
			return "", fmt.Errorf("%w: 未配置BAILIAN_API_KEY", ErrTTSAPIKeyMissing)
		}
		utils.LogInfo("🎵 使用系统预设音色API密钥 - 长度: %d", len(apiKey))
	}

	return apiKey, nil
}

// connectTTSWebSocket 连接TTS WebSocket
func connectTTSWebSocket(apiKey string) (*websocket.Conn, error) {
	header := make(http.Header)
	header.Add("X-DashScope-DataInspection", "enable")
	header.Add("Authorization", fmt.Sprintf("bearer %s", apiKey))

	// 配置WebSocket拨号器
	dialer := &websocket.Dialer{
		HandshakeTimeout: TTSDialTimeout,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	utils.LogInfo("🔗 正在连接CosyVoice WebSocket服务...")
	conn, _, err := dialer.Dial(CosyVoiceWSURL, header)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTTSConnectionFailed, err)
	}

	// 设置超时
	if err := conn.SetReadDeadline(time.Now().Add(TTSReadTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置读取超时失败: %v", err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(TTSWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	return conn, nil
}

// generateTTSTaskID 生成TTS任务ID
func generateTTSTaskID() string {
	return uuid.New().String()
}

// sendRunTaskCmd 发送run-task指令
func (s *StreamTTSService) sendRunTaskCmd(voiceModel string) error {
	// 如果没有指定音色，使用默认音色
	if voiceModel == "" {
		voiceModel = DefaultVoiceModel
	}

	runTaskCmd := TTSEvent{
		Header: TTSHeader{
			Action:    "run-task",
			TaskID:    s.taskID,
			Streaming: TTSStreamingMode,
		},
		Payload: TTSPayload{
			TaskGroup: "audio",
			Task:      "tts",
			Function:  TTSFunction,
			Model:     TTSModel,
			Parameters: TTSParams{
				TextType:   TTSTextType,
				Voice:      voiceModel,
				Format:     TTSFormat,
				SampleRate: TTSSampleRate,
				Volume:     TTSVolume,
				Rate:       TTSRate,
				Pitch:      TTSPitch,
			},
			Input: TTSInput{},
		},
	}

	cmdJSON, err := json.Marshal(runTaskCmd)
	if err != nil {
		return fmt.Errorf("run-task指令序列化失败: %v", err)
	}

	utils.LogDebug("📤 发送run-task指令: %s", string(cmdJSON))

	// 设置写入超时
	if err := s.conn.SetWriteDeadline(time.Now().Add(TTSWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, cmdJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrTTSTaskStartFailed, err)
	}

	return nil
}

// SendText 发送continue-task指令
func (s *StreamTTSService) SendText(text string) error {
	if !s.isConnected {
		return errors.New("WebSocket未连接")
	}

	continueTaskCmd := TTSEvent{
		Header: TTSHeader{
			Action:    "continue-task",
			TaskID:    s.taskID,
			Streaming: TTSStreamingMode,
		},
		Payload: TTSPayload{
			Input: TTSInput{
				Text: text,
			},
		},
	}

	cmdJSON, err := json.Marshal(continueTaskCmd)
	if err != nil {
		return fmt.Errorf("continue-task指令序列化失败: %v", err)
	}

	utils.LogDebug("📤 发送continue-task指令: %s", string(cmdJSON))

	// 设置写入超时
	if err := s.conn.SetWriteDeadline(time.Now().Add(TTSWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, cmdJSON); err != nil {
		utils.LogError("发送文本失败: %v", err)
		return fmt.Errorf("%w: %v", ErrTTSTextSendFailed, err)
	}

	return nil
}

// FinishTask 发送finish-task指令
func (s *StreamTTSService) FinishTask() error {
	if !s.isConnected {
		return errors.New("WebSocket未连接")
	}

	finishTaskCmd := TTSEvent{
		Header: TTSHeader{
			Action:    "finish-task",
			TaskID:    s.taskID,
			Streaming: TTSStreamingMode,
		},
		Payload: TTSPayload{
			Input: TTSInput{},
		},
	}

	cmdJSON, err := json.Marshal(finishTaskCmd)
	if err != nil {
		return fmt.Errorf("finish-task指令序列化失败: %v", err)
	}

	utils.LogInfo("📤 发送finish-task指令: %s", string(cmdJSON))

	// 设置写入超时
	if err := s.conn.SetWriteDeadline(time.Now().Add(TTSWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, cmdJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrTTSTaskFinishFailed, err)
	}

	return nil
}

// receiveAudio 接收音频数据
func (s *StreamTTSService) receiveAudio() {
	defer close(s.audioChan)
	taskStarted := false

	for {
		// 设置读取超时
		if err := s.conn.SetReadDeadline(time.Now().Add(TTSReadTimeout)); err != nil {
			utils.LogWarn("⚠️ 设置读取超时失败: %v", err)
		}

		msgType, message, err := s.conn.ReadMessage()
		if err != nil {
			utils.LogError("TTS WebSocket消息读取失败: %v", err)
			return
		}

		if msgType == websocket.BinaryMessage {
			// 接收到音频数据
			if taskStarted {
				s.handleAudioData(message)
			}
		} else {
			// 处理文本事件
			taskStarted = s.handleTextEvent(message, taskStarted)
		}
	}
}

// handleAudioData 处理音频数据
func (s *StreamTTSService) handleAudioData(message []byte) {
	utils.LogDebug("接收到音频数据，大小: %d bytes", len(message))

	select {
	case s.audioChan <- message:
		// 成功发送到通道
	default:
		utils.LogWarn("音频通道已满，丢弃数据")
	}
}

// handleTextEvent 处理文本事件
func (s *StreamTTSService) handleTextEvent(message []byte, taskStarted bool) bool {
	var event TTSEvent
	if err := json.Unmarshal(message, &event); err != nil {
		utils.LogError("TTS事件解析失败: %v", err)
		return taskStarted
	}

	utils.LogDebug("收到TTS事件: %s", event.Header.Event)

	switch event.Header.Event {
	case "task-started":
		utils.LogInfo("TTS任务已启动")
		return true

	case "result-generated":
		// 忽略result-generated事件
		return taskStarted

	case "task-finished":
		utils.LogInfo("TTS任务完成")
		return taskStarted

	case "task-failed":
		utils.LogError("TTS任务失败: %s - %s", event.Header.ErrorCode, event.Header.ErrorMessage)
		return taskStarted

	default:
		utils.LogWarn("未知的TTS事件类型: %s", event.Header.Event)
		return taskStarted
	}
}

// GetAudioChannel 获取音频通道
func (s *StreamTTSService) GetAudioChannel() <-chan []byte {
	return s.audioChan
}

// Close 关闭连接
func (s *StreamTTSService) Close() error {
	if s.conn != nil {
		s.isConnected = false
		utils.LogInfo("TTS WebSocket连接已关闭")
		if err := s.conn.Close(); err != nil {
			utils.LogWarn("⚠️ 关闭WebSocket连接失败: %v", err)
			return err
		}
	}
	return nil
}
