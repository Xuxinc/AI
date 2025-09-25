package services

import (
	"ai-celebrity-simulator/utils"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WebSocket配置常量
const (
	ASRDialTimeout  = 10 * time.Second
	ASRWriteTimeout = 10 * time.Second
	ASRReadTimeout  = 60 * time.Second
)

// ASR模型配置常量
const (
	ASRModel         = "paraformer-realtime-v2"
	ASRFormat        = "pcm"
	ASRSampleRate    = 16000
	ASRLanguageHint  = "zh"
	ASRStreamingMode = "duplex"
)

// 错误定义
var (
	ErrASRAPIKeyMissing    = errors.New("ASR API密钥未配置")
	ErrASRConnectionFailed = errors.New("ASR WebSocket连接失败")
	ErrASRTaskCreateFailed = errors.New("ASR任务创建失败")
	ErrASRTaskStartFailed  = errors.New("ASR任务启动失败")
	ErrASRDataSendFailed   = errors.New("ASR音频数据发送失败")
	ErrASRTaskFinishFailed = errors.New("ASR任务结束失败")
)

const (
	AliyunASRWsURL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"
)

// 语音识别WebSocket连接
// 支持流式音频推送和识别结果回调

type VoiceRecognitionService struct {
	conn        *websocket.Conn
	taskID      string
	resultChan  chan<- string
	isConnected bool
}

type asrHeader struct {
	Action       string                 `json:"action"`
	TaskID       string                 `json:"task_id"`
	Streaming    string                 `json:"streaming"`
	Event        string                 `json:"event,omitempty"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
}

type asrParams struct {
	Format        string   `json:"format"`
	SampleRate    int      `json:"sample_rate"`
	LanguageHints []string `json:"language_hints"`
	Heartbeat     bool     `json:"heartbeat"`
}

type asrPayload struct {
	TaskGroup  string    `json:"task_group"`
	Task       string    `json:"task"`
	Function   string    `json:"function"`
	Model      string    `json:"model"`
	Parameters asrParams `json:"parameters"`
	Input      struct{}  `json:"input"`
}

type asrEvent struct {
	Header  asrHeader  `json:"header"`
	Payload asrPayload `json:"payload"`
}

type asrSentence struct {
	BeginTime int64  `json:"begin_time"`
	EndTime   *int64 `json:"end_time"`
	Text      string `json:"text"`
	Words     []struct {
		BeginTime   int64  `json:"begin_time"`
		EndTime     *int64 `json:"end_time"`
		Text        string `json:"text"`
		Punctuation string `json:"punctuation"`
	} `json:"words"`
}

type asrOutput struct {
	Sentence asrSentence `json:"sentence"`
	Usage    interface{} `json:"usage"`
}

type asrResultPayload struct {
	Output asrOutput `json:"output"`
}

type asrResultEvent struct {
	Header  asrHeader        `json:"header"`
	Payload asrResultPayload `json:"payload"`
}

// NewVoiceRecognitionService 创建语音识别服务（连接WebSocket并发送run-task指令）
func NewVoiceRecognitionService() (*VoiceRecognitionService, error) {
	utils.LogInfo("🎤 开始创建语音识别服务...")

	startTime := time.Now()

	// 获取并验证API密钥
	apiKey, err := getASRAPIKey()
	if err != nil {
		utils.LogError("❌ 语音识别服务初始化失败: %v", err)
		return nil, err
	}

	// 建立WebSocket连接
	conn, err := connectASRWebSocket(apiKey)
	if err != nil {
		utils.LogError("❌ ASR WebSocket连接失败: %v", err)
		return nil, err
	}

	connectionTime := time.Since(startTime)
	utils.LogInfo("✅ ASR WebSocket连接建立成功 - 耗时: %v", connectionTime)

	// 生成任务ID
	taskID := generateASRTaskID()
	utils.LogInfo("🆔 生成ASR任务ID: %s", taskID)

	// 创建服务实例
	service := &VoiceRecognitionService{
		conn:        conn,
		taskID:      taskID,
		isConnected: true,
	}

	// 发送run-task指令
	if err := service.sendRunTaskCommand(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭WebSocket连接失败: %v", closeErr)
		}
		utils.LogError("❌ 发送run-task指令失败: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrASRTaskCreateFailed, err)
	}

	utils.LogInfo("✅ run-task指令发送成功")
	return service, nil
}

// getASRAPIKey 获取ASR API密钥
func getASRAPIKey() (string, error) {
	apiKey := os.Getenv("BAILIAN_API_KEY")
	if apiKey == "" {
		return "", ErrASRAPIKeyMissing
	}

	// 脱敏记录API密钥信息
	utils.LogInfo("🔑 使用ASR API密钥 - 长度: %d", len(apiKey))
	return apiKey, nil
}

// connectASRWebSocket 连接ASR WebSocket
func connectASRWebSocket(apiKey string) (*websocket.Conn, error) {
	header := make(http.Header)
	header.Add("X-DashScope-DataInspection", "enable")
	header.Add("Authorization", fmt.Sprintf("bearer %s", apiKey))

	// 配置WebSocket拨号器
	dialer := &websocket.Dialer{
		HandshakeTimeout: ASRDialTimeout,
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
	}

	utils.LogInfo("🔗 正在连接阿里云ASR WebSocket服务...")
	conn, _, err := dialer.Dial(AliyunASRWsURL, header)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrASRConnectionFailed, err)
	}

	// 设置超时
	if err := conn.SetReadDeadline(time.Now().Add(ASRReadTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置读取超时失败: %v", err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(ASRWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	return conn, nil
}

// generateASRTaskID 生成ASR任务ID
func generateASRTaskID() string {
	return uuid.New().String()
}

// sendRunTaskCommand 发送run-task指令
func (s *VoiceRecognitionService) sendRunTaskCommand() error {
	runTaskCmd := asrEvent{
		Header: asrHeader{
			Action:    "run-task",
			TaskID:    s.taskID,
			Streaming: ASRStreamingMode,
		},
		Payload: asrPayload{
			TaskGroup: "audio",
			Task:      "asr",
			Function:  "recognition",
			Model:     ASRModel,
			Parameters: asrParams{
				Format:        ASRFormat,
				SampleRate:    ASRSampleRate,
				LanguageHints: []string{ASRLanguageHint},
				Heartbeat:     true,
			},
			Input: struct{}{},
		},
	}

	cmdJSON, err := json.Marshal(runTaskCmd)
	if err != nil {
		return fmt.Errorf("run-task指令序列化失败: %v", err)
	}

	utils.LogDebug("📤 发送run-task指令: %s", string(cmdJSON))

	// 设置写入超时
	if err := s.conn.SetWriteDeadline(time.Now().Add(ASRWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, cmdJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrASRTaskStartFailed, err)
	}

	return nil
}

// SendAudioData 发送音频数据
func (s *VoiceRecognitionService) SendAudioData(audioData []byte) error {
	if !s.isConnected {
		return errors.New("WebSocket未连接")
	}

	// 设置写入超时
	if err := s.conn.SetWriteDeadline(time.Now().Add(ASRWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := s.conn.WriteMessage(websocket.BinaryMessage, audioData); err != nil {
		utils.LogError("发送音频数据失败: %v", err)
		return fmt.Errorf("%w: %v", ErrASRDataSendFailed, err)
	}

	return nil
}

// FinishTask 结束语音识别任务
func (s *VoiceRecognitionService) FinishTask() error {
	if !s.isConnected {
		return errors.New("WebSocket未连接")
	}

	finishTaskCmd := asrEvent{
		Header: asrHeader{
			Action:    "finish-task",
			TaskID:    s.taskID,
			Streaming: ASRStreamingMode,
		},
		Payload: asrPayload{
			Input: struct{}{},
		},
	}

	cmdJSON, err := json.Marshal(finishTaskCmd)
	if err != nil {
		return fmt.Errorf("finish-task指令序列化失败: %v", err)
	}

	utils.LogInfo("📤 发送finish-task指令: %s", string(cmdJSON))

	// 设置写入超时
	if err := s.conn.SetWriteDeadline(time.Now().Add(ASRWriteTimeout)); err != nil {
		utils.LogWarn("⚠️ 设置写入超时失败: %v", err)
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, cmdJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrASRTaskFinishFailed, err)
	}

	return nil
}

// ListenForResults 监听识别结果（异步）
func (s *VoiceRecognitionService) ListenForResults(resultChan chan<- string, startedChan chan<- struct{}) {
	go func() {
		defer s.closeChannels(resultChan, startedChan)

		for {
			// 设置读取超时
			if err := s.conn.SetReadDeadline(time.Now().Add(ASRReadTimeout)); err != nil {
				utils.LogWarn("⚠️ 设置读取超时失败: %v", err)
			}

			_, message, err := s.conn.ReadMessage()
			if err != nil {
				utils.LogError("ASR WebSocket消息读取失败: %v", err)
				return
			}

			utils.LogDebug("收到ASR服务原始消息: %s", string(message))

			var event asrResultEvent
			if err := json.Unmarshal(message, &event); err != nil {
				utils.LogError("ASR事件解析失败: %v", err)
				continue
			}

			utils.LogDebug("收到事件类型: %s", event.Header.Event)
			s.handleASREvent(event, resultChan, startedChan)
		}
	}()
}

// closeChannels 关闭通道
func (s *VoiceRecognitionService) closeChannels(resultChan chan<- string, startedChan chan<- struct{}) {
	defer func() {
		if r := recover(); r != nil {
			utils.LogError("关闭通道时发生panic: %v", r)
		}
	}()

	if resultChan != nil {
		close(resultChan)
	}
	if startedChan != nil {
		close(startedChan)
	}
}

// handleASREvent 处理ASR事件
func (s *VoiceRecognitionService) handleASREvent(event asrResultEvent, resultChan chan<- string, startedChan chan<- struct{}) {
	switch event.Header.Event {
	case "task-started":
		utils.LogInfo("ASR任务已启动")
		s.handleTaskStarted(startedChan)

	case "result-generated":
		s.handleResultGenerated(event, resultChan)

	case "task-finished":
		utils.LogInfo("ASR任务完成")
		s.handleTaskFinished(event, resultChan)

	case "task-failed":
		utils.LogError("ASR任务失败: %s - %s", event.Header.ErrorCode, event.Header.ErrorMessage)

	default:
		utils.LogWarn("未知的ASR事件类型: %s", event.Header.Event)
	}
}

// handleTaskStarted 处理任务启动事件
func (s *VoiceRecognitionService) handleTaskStarted(startedChan chan<- struct{}) {
	if startedChan != nil {
		select {
		case startedChan <- struct{}{}:
			utils.LogInfo("已发送task-started信号")
		default:
			utils.LogWarn("startedChan已满，跳过发送")
		}
	}
}

// handleResultGenerated 处理结果生成事件
func (s *VoiceRecognitionService) handleResultGenerated(event asrResultEvent, resultChan chan<- string) {
	sentence := event.Payload.Output.Sentence
	utils.LogDebug("ASR识别结果: %+v", sentence)

	if sentence.Text != "" {
		// 检查是否为最终结果
		isFinalResult := sentence.EndTime != nil

		// 构造结果消息，包含是否为最终结果的标识
		resultMessage := fmt.Sprintf("FINAL:%t:TEXT:%s", isFinalResult, sentence.Text)

		utils.LogInfo("发送识别结果到通道，是否为最终结果: %t, 文本长度: %d", isFinalResult, len(sentence.Text))

		select {
		case resultChan <- resultMessage:
			// 成功发送
		default:
			utils.LogWarn("结果通道已满，丢弃识别结果")
		}
	}
}

// handleTaskFinished 处理任务完成事件
func (s *VoiceRecognitionService) handleTaskFinished(event asrResultEvent, resultChan chan<- string) {
	// 检查是否有错误信息
	if event.Header.ErrorCode != "" {
		utils.LogError("ASR任务完成但有错误: %s - %s", event.Header.ErrorCode, event.Header.ErrorMessage)
	}

	// 检查payload中是否有输出
	if event.Payload.Output.Sentence.Text != "" {
		utils.LogInfo("ASR任务完成时发现最终结果，文本长度: %d", len(event.Payload.Output.Sentence.Text))
		resultMessage := fmt.Sprintf("FINAL:true:TEXT:%s", event.Payload.Output.Sentence.Text)

		select {
		case resultChan <- resultMessage:
			// 成功发送
		default:
			utils.LogWarn("结果通道已满，丢弃最终结果")
		}
	} else {
		utils.LogWarn("ASR任务完成但没有识别结果")
	}
}

// Conn 导出WebSocket连接，便于外部访问
func (s *VoiceRecognitionService) Conn() *websocket.Conn {
	return s.conn
}

// Close 关闭连接
func (s *VoiceRecognitionService) Close() error {
	if s.conn != nil {
		s.isConnected = false
		utils.LogInfo("ASR WebSocket连接已关闭")
		return s.conn.Close()
	}
	return nil
}
