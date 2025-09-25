package services

import (
	"ai-celebrity-simulator/config"
	"ai-celebrity-simulator/database"
	"ai-celebrity-simulator/models"
	"ai-celebrity-simulator/utils"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultHTTPTimeout HTTP客户端配置常量
const (
	DefaultHTTPTimeout = 30 * time.Second
)

// AI模型配置常量
const (
	DefaultModel       = "qwen-plus"
	VisionModel        = "qwen-vl-plus"
	DefaultMaxTokens   = 1000
	DefaultTemperature = 0.8
	DefaultTopP        = 0.9
	MaxHistoryMessages = 20
)

// StatusOK HTTP状态码常量
const (
	StatusOK = http.StatusOK
)

// MessageAIUnavailable 响应消息常量
const (
	MessageAIUnavailable = "当前AI对话不可用"
)

// AI提示词模板常量
const (
	// SystemPromptTemplate 文本模型系统提示词模板
	SystemPromptTemplate = `你是一个AI名人模拟器。请严格按照以下角色设定来回复：
%s
重要规则：
1. 始终以这个名人的身份和语气回复
2. 使用中文回复
3. 回复要自然、有趣、符合人物特点
4. 回复长度控制在100-200字之间
5. 不要暴露你是AI的身份
6. 保持对话的连贯性，记住之前的对话内容`

	// VisionSystemPromptTemplate 视觉模型系统提示词模板
	VisionSystemPromptTemplate = `你是一个AI名人模拟器。请严格按照以下角色设定来回复：
%s
重要规则：
1. 始终以这个名人的身份和语气回复
2. 使用中文回复
3. 回复要自然、符合人物特点
4. 回复长度控制在100-200字之间
5. 不要暴露你是AI的身份
6. 保持对话的连贯性，记住之前的对话内容
7. 如果用户发送了图片，请仔细观察图片内容并结合你的角色身份进行回应
8. 对图片的分析要准确、生动，体现你的角色特点
9. 在回复中自然地融入图片内容，不要生硬地描述图片

用户消息：%s`
)

// 错误定义
var (
	ErrEmptyResponse      = errors.New("AI响应为空")
	ErrInvalidResponse    = errors.New("AI响应格式无效")
	ErrRequestTimeout     = errors.New("请求超时")
	ErrServiceUnavailable = errors.New("AI服务不可用")
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Input struct {
	Messages []Message `json:"messages"`
}

type Parameters struct {
	MaxTokens   int     `json:"max_tokens"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
}

// BailianStreamRequest 阿里百炼流式API请求结构
type BailianStreamRequest struct {
	Model      string     `json:"model"`
	Input      Input      `json:"input"`
	Parameters Parameters `json:"parameters"`
	Stream     bool       `json:"stream"`
}

// BailianStreamResponse 阿里百炼流式API响应结构
type BailianStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// GenerateAIReplyWithHistory 生成AI回复（支持多轮对话）
func GenerateAIReplyWithHistory(dialogID uint, characterPrompt, userMessage string) (string, error) {
	// 验证输入参数
	if err := validateAIRequest(characterPrompt, userMessage); err != nil {
		utils.LogError("AI请求参数验证失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return MessageAIUnavailable, nil
	}

	// 获取历史对话记录
	historyMessages, err := getDialogHistory(dialogID)
	if err != nil {
		utils.LogError("获取历史对话失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return MessageAIUnavailable, nil
	}

	utils.LogInfo("开始生成AI回复 - 会话ID: %d, 历史消息数: %d", dialogID, len(historyMessages))

	// 构建请求消息
	messages, err := buildMessages(characterPrompt, userMessage, historyMessages)
	if err != nil {
		utils.LogError("构建消息失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return MessageAIUnavailable, nil
	}

	// 调用AI API
	reply, err := callAIAPI(messages, false)
	if err != nil {
		utils.LogError("AI API调用失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return MessageAIUnavailable, nil
	}

	utils.LogInfo("AI回复生成成功 - 会话ID: %d, 回复长度: %d", dialogID, len(reply))
	return reply, nil
}

// validateAIRequest 验证AI请求参数
func validateAIRequest(characterPrompt, userMessage string) error {
	if strings.TrimSpace(characterPrompt) == "" {
		return errors.New("角色提示词不能为空")
	}
	if strings.TrimSpace(userMessage) == "" {
		return errors.New("用户消息不能为空")
	}
	return nil
}

// buildMessages 构建请求消息
func buildMessages(characterPrompt, userMessage string, historyMessages []models.Message) ([]Message, error) {
	// 构建完整的提示词
	systemPrompt := fmt.Sprintf(SystemPromptTemplate, characterPrompt)

	// 构建请求消息（包含历史对话）
	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	// 添加历史对话（最多保留最近的消息）
	if len(historyMessages) > MaxHistoryMessages {
		historyMessages = historyMessages[len(historyMessages)-MaxHistoryMessages:]
	}

	for _, msg := range historyMessages {
		// 将数据库中的角色映射为API要求的角色
		apiRole := msg.Role
		if msg.Role == "ai" {
			apiRole = "assistant"
		}
		messages = append(messages, Message{
			Role:    apiRole,
			Content: msg.Content,
		})
	}

	// 添加当前用户消息
	messages = append(messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	return messages, nil
}

// callAIAPI 调用AI API
func callAIAPI(messages []Message, stream bool) (string, error) {
	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       DefaultModel,
		"messages":    messages,
		"max_tokens":  DefaultMaxTokens,
		"temperature": DefaultTemperature,
		"top_p":       DefaultTopP,
		"stream":      stream,
	}

	// 序列化请求体
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %v", err)
	}

	utils.LogInfo("发送AI API请求 - URL: %s", config.AppConfig.BailianBaseURL+"/chat/completions")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTimeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.BailianBaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.BailianAPIKey)

	// 发送请求
	client := &http.Client{Timeout: DefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", ErrRequestTimeout
		}
		return "", fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	if resp == nil {
		return "", fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	utils.LogInfo("AI API响应 - 状态码: %d, 响应长度: %d", resp.StatusCode, len(body))

	// 检查响应状态
	if resp.StatusCode != StatusOK {
		utils.LogError("AI API响应状态码错误 - 状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return "", ErrServiceUnavailable
	}

	// 解析响应
	return parseAIResponse(body)
}

// parseAIResponse 解析AI响应
func parseAIResponse(body []byte) (string, error) {
	var openaiResponse map[string]interface{}
	if err := json.Unmarshal(body, &openaiResponse); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查错误
	if errorInfo, exists := openaiResponse["error"]; exists {
		utils.LogError("AI API返回错误: %+v", errorInfo)
		return "", ErrServiceUnavailable
	}

	// 获取回复内容
	if choices, exists := openaiResponse["choices"].([]interface{}); exists && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, exists := choice["message"].(map[string]interface{}); exists {
				if content, exists := message["content"].(string); exists {
					if strings.TrimSpace(content) == "" {
						return "", ErrEmptyResponse
					}
					return content, nil
				}
			}
		}
	}

	return "", ErrInvalidResponse
}

// getDialogHistory 获取对话历史记录
func getDialogHistory(dialogID uint) ([]models.Message, error) {
	var messages []models.Message
	err := database.DB.Where("dialog_id = ? AND is_deleted = ?", dialogID, "no").
		Order("time ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("查询历史消息失败: %v", err)
	}

	// 过滤掉通话时长消息
	var filteredMessages []models.Message
	for _, msg := range messages {
		if !strings.Contains(msg.Content, "通话时长:") {
			filteredMessages = append(filteredMessages, msg)
		}
	}

	return filteredMessages, nil
}

// VisionContent 新增：支持图片的消息内容结构
type VisionContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

type VisionMessage struct {
	Role    string          `json:"role"`
	Content []VisionContent `json:"content"`
}

type VisionRequest struct {
	Model    string          `json:"model"`
	Messages []VisionMessage `json:"messages"`
}

// CallAIWithImages 调用支持图片的AI模型（支持多轮对话）
func CallAIWithImages(dialogID uint, characterPrompt, userMessage string, imageUrls []string) (string, error) {
	// 验证输入参数
	if err := validateVisionRequest(characterPrompt, userMessage, imageUrls); err != nil {
		utils.LogError("视觉AI请求参数验证失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return "", err
	}

	utils.LogInfo("开始调用视觉AI模型 - 会话ID: %d, 图片数量: %d", dialogID, len(imageUrls))

	// 获取历史消息
	historyMessages, err := getDialogHistory(dialogID)
	if err != nil {
		utils.LogError("获取历史对话失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return "", fmt.Errorf("获取历史对话失败: %v", err)
	}

	// 构建视觉消息
	messages, err := buildVisionMessages(characterPrompt, userMessage, imageUrls, historyMessages)
	if err != nil {
		utils.LogError("构建视觉消息失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return "", err
	}

	// 调用视觉API
	reply, err := callVisionAPI(messages)
	if err != nil {
		utils.LogError("视觉AI API调用失败 - 会话ID: %d, 错误: %v", dialogID, err)
		return "", err
	}

	utils.LogInfo("视觉AI回复生成成功 - 会话ID: %d, 回复长度: %d", dialogID, len(reply))
	return reply, nil
}

// validateVisionRequest 验证视觉AI请求参数
func validateVisionRequest(characterPrompt, userMessage string, imageUrls []string) error {
	if strings.TrimSpace(characterPrompt) == "" {
		return errors.New("角色提示词不能为空")
	}
	if strings.TrimSpace(userMessage) == "" && len(imageUrls) == 0 {
		return errors.New("用户消息和图片不能同时为空")
	}
	return nil
}

// buildVisionMessages 构建视觉消息
func buildVisionMessages(characterPrompt, userMessage string, imageUrls []string, historyMessages []models.Message) ([]VisionMessage, error) {
	// 构建完整的提示词
	systemPrompt := fmt.Sprintf(VisionSystemPromptTemplate, characterPrompt, userMessage)

	// 限制历史消息数量
	if len(historyMessages) > MaxHistoryMessages {
		historyMessages = historyMessages[len(historyMessages)-MaxHistoryMessages:]
	}

	// 构建消息列表
	var messages []VisionMessage

	// 添加历史消息（转换为视觉消息格式）
	for _, msg := range historyMessages {
		if msg.Role == "user" {
			userContent := buildUserVisionContent(msg)
			messages = append(messages, VisionMessage{
				Role:    "user",
				Content: userContent,
			})
		} else if msg.Role == "ai" {
			messages = append(messages, VisionMessage{
				Role: "assistant",
				Content: []VisionContent{
					{
						Type: "text",
						Text: msg.Content,
					},
				},
			})
		}
	}

	// 添加当前用户消息（包含系统提示词）
	currentUserContent := buildCurrentVisionContent(imageUrls, systemPrompt)
	messages = append(messages, VisionMessage{
		Role:    "user",
		Content: currentUserContent,
	})

	return messages, nil
}

// buildUserVisionContent 构建用户视觉内容
func buildUserVisionContent(msg models.Message) []VisionContent {
	var userContent []VisionContent

	// 如果有图片，添加图片内容
	if msg.PictureURL != "" {
		imageUrls := strings.Split(msg.PictureURL, ",")
		for _, imageUrl := range imageUrls {
			imageUrl = strings.TrimSpace(imageUrl)
			if imageUrl != "" {
				userContent = append(userContent, VisionContent{
					Type: "image_url",
					ImageURL: &struct {
						URL string `json:"url"`
					}{
						URL: imageUrl,
					},
				})
			}
		}
	}

	// 添加文本内容
	if msg.Content != "" {
		userContent = append(userContent, VisionContent{
			Type: "text",
			Text: msg.Content,
		})
	}

	return userContent
}

// buildCurrentVisionContent 构建当前视觉内容
func buildCurrentVisionContent(imageUrls []string, systemPrompt string) []VisionContent {
	var currentUserContent []VisionContent

	// 添加当前消息的图片内容
	for _, imageUrl := range imageUrls {
		currentUserContent = append(currentUserContent, VisionContent{
			Type: "image_url",
			ImageURL: &struct {
				URL string `json:"url"`
			}{
				URL: imageUrl,
			},
		})
	}

	// 添加当前消息的文本内容（包含系统提示词）
	currentUserContent = append(currentUserContent, VisionContent{
		Type: "text",
		Text: systemPrompt,
	})

	return currentUserContent
}

// callVisionAPI 调用视觉API
func callVisionAPI(messages []VisionMessage) (string, error) {
	// 构建请求
	request := VisionRequest{
		Model:    VisionModel,
		Messages: messages,
	}

	// 发送请求
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("序列化视觉请求失败: %v", err)
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.BailianBaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建视觉请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 选择API密钥
	apiKey := selectVisionAPIKey()
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: DefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			utils.LogError("视觉AI API请求超时")
			return "", ErrRequestTimeout
		}
		return "", fmt.Errorf("发送视觉请求失败: %v", err)
	}
	if resp == nil {
		return "", fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	if resp.StatusCode != StatusOK {
		body, _ := io.ReadAll(resp.Body)
		utils.LogError("视觉AI API请求失败 - 状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return "", ErrServiceUnavailable
	}

	// 解析响应
	return parseVisionResponse(resp.Body)
}

// selectVisionAPIKey 选择视觉API密钥
func selectVisionAPIKey() string {
	// 使用第二个API密钥（视觉模型专用）
	apiKey := config.AppConfig.BailianAPIKey2
	if apiKey == "" {
		apiKey = config.AppConfig.BailianAPIKey // 如果没有第二个密钥，使用第一个
		utils.LogInfo("视觉模型API调用：使用备用API密钥（BAILIAN_API_KEY）")
	} else {
		utils.LogInfo("视觉模型API调用：使用专用API密钥（BAILIAN_API_KEY_2）")
	}
	return apiKey
}

// parseVisionResponse 解析视觉响应
func parseVisionResponse(body io.Reader) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error map[string]interface{} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return "", fmt.Errorf("解析视觉响应失败: %v", err)
	}

	// 检查错误
	if response.Error != nil {
		utils.LogError("视觉AI API返回错误: %+v", response.Error)
		return "", ErrServiceUnavailable
	}

	if len(response.Choices) == 0 {
		return "", ErrEmptyResponse
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return "", ErrEmptyResponse
	}

	return content, nil
}
