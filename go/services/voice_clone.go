package services

import (
	"ai-celebrity-simulator/utils"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// HTTP客户端配置常量
const (
	VoiceCloneHTTPTimeout = 30 * time.Second
	APIKeyMinLength       = 10
	MaxAPIRetries         = 3
	APIRetryDelay         = 2 * time.Second
)

// API配置常量
const (
	VoiceCloneModel       = "voice-enrollment"
	VoiceCloneTargetModel = "cosyvoice-v2"
	VoiceCloneAPIURL      = "https://dashscope.aliyuncs.com/api/v1/services/audio/tts/customization"
)

// VoiceCloneStatusOK HTTP状态码常量
const (
	VoiceCloneStatusOK = http.StatusOK
)

// 错误定义
var (
	ErrAPIKeyNotConfigured  = errors.New("API密钥未配置")
	ErrRequestMarshalFailed = errors.New("序列化请求数据失败")
	ErrHTTPRequestFailed    = errors.New("HTTP请求失败")
	ErrAPIResponseError     = errors.New("API响应错误")
	ErrVoiceIDNotFound      = errors.New("响应中未找到voice_id")
)

// VoiceCloneRequest 音色克隆请求结构
type VoiceCloneRequest struct {
	Model string                 `json:"model"`
	Input map[string]interface{} `json:"input"`
}

// VoiceCloneResponse 音色克隆响应结构
type VoiceCloneResponse struct {
	Output map[string]interface{} `json:"output"`
	Error  map[string]interface{} `json:"error,omitempty"`
}

// CreateVoiceModel 创建音色模型
func CreateVoiceModel(audioURL, prefix string) (string, error) {
	utils.LogInfo("开始创建音色模型 - 前缀长度: %d", len(prefix))

	// 验证输入参数
	if err := validateVoiceCloneParams(audioURL, prefix); err != nil {
		utils.LogError("音色克隆参数验证失败: %v", err)
		return "", err
	}

	// 获取并验证API密钥
	apiKey, err := getAndValidateAPIKey()
	if err != nil {
		utils.LogError("API密钥验证失败: %v", err)
		return "", err
	}

	// 构建请求
	request := buildVoiceCloneRequest(audioURL, prefix)

	// 发送请求
	voiceID, err := sendVoiceCloneRequest(request, apiKey)
	if err != nil {
		utils.LogError("音色克隆请求失败: %v", err)
		return "", err
	}

	utils.LogInfo("音色模型创建成功 - VoiceID长度: %d", len(voiceID))
	return voiceID, nil
}

// validateVoiceCloneParams 验证音色克隆参数
func validateVoiceCloneParams(audioURL, prefix string) error {
	if audioURL == "" {
		return errors.New("音频URL不能为空")
	}
	if prefix == "" {
		return errors.New("音色前缀不能为空")
	}
	return nil
}

// getAndValidateAPIKey 获取并验证API密钥
func getAndValidateAPIKey() (string, error) {
	apiKey := os.Getenv("BAILIAN_API_KEY_2")
	if apiKey == "" {
		return "", ErrAPIKeyNotConfigured
	}

	// 脱敏记录API密钥信息
	if len(apiKey) > APIKeyMinLength {
		utils.LogInfo("使用API密钥 - 前缀: %s..., 长度: %d", apiKey[:APIKeyMinLength], len(apiKey))
	} else {
		utils.LogInfo("使用API密钥 - 长度: %d", len(apiKey))
	}

	return apiKey, nil
}

// buildVoiceCloneRequest 构建音色克隆请求
func buildVoiceCloneRequest(audioURL, prefix string) VoiceCloneRequest {
	return VoiceCloneRequest{
		Model: VoiceCloneModel,
		Input: map[string]interface{}{
			"action":       "create_voice",
			"target_model": VoiceCloneTargetModel,
			"prefix":       prefix,
			"url":          audioURL,
		},
	}
}

// sendVoiceCloneRequest 发送音色克隆请求
func sendVoiceCloneRequest(request VoiceCloneRequest, apiKey string) (string, error) {
	// 序列化请求数据
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrRequestMarshalFailed, err)
	}

	utils.LogInfo("发送音色克隆API请求 - URL: %s", VoiceCloneAPIURL)

	// 带重试的请求发送
	return sendRequestWithRetry(jsonData, apiKey)
}

// sendRequestWithRetry 带重试的请求发送
func sendRequestWithRetry(jsonData []byte, apiKey string) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= MaxAPIRetries; attempt++ {
		utils.LogInfo("尝试发送音色克隆请求 - 第 %d/%d 次", attempt, MaxAPIRetries)

		voiceID, err := sendSingleRequest(jsonData, apiKey)
		if err == nil {
			utils.LogInfo("音色克隆请求成功")
			return voiceID, nil
		}

		lastErr = err
		utils.LogWarn("第 %d 次请求失败: %v", attempt, err)

		if attempt < MaxAPIRetries {
			utils.LogInfo("等待 %v 后重试", APIRetryDelay)
			time.Sleep(APIRetryDelay)
		}
	}

	return "", fmt.Errorf("请求失败，已重试 %d 次: %v", MaxAPIRetries, lastErr)
}

// sendSingleRequest 发送单次请求
func sendSingleRequest(jsonData []byte, apiKey string) (string, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), VoiceCloneHTTPTimeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", VoiceCloneAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{Timeout: VoiceCloneHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("请求超时: %v", err)
		}
		return "", fmt.Errorf("%w: %v", ErrHTTPRequestFailed, err)
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

	utils.LogInfo("音色克隆API响应 - 状态码: %d, 响应长度: %d", resp.StatusCode, len(body))

	// 检查响应状态
	if resp.StatusCode != VoiceCloneStatusOK {
		utils.LogError("API响应状态码错误 - 状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("%w: 状态码 %d", ErrAPIResponseError, resp.StatusCode)
	}

	// 解析响应
	return parseVoiceCloneResponse(body)
}

// parseVoiceCloneResponse 解析音色克隆响应
func parseVoiceCloneResponse(body []byte) (string, error) {
	var response VoiceCloneResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查错误
	if response.Error != nil {
		utils.LogError("API返回错误: %+v", response.Error)
		return "", ErrAPIResponseError
	}

	// 提取voice_id
	if voiceID, ok := response.Output["voice_id"].(string); ok && voiceID != "" {
		return voiceID, nil
	}

	utils.LogError("响应中未找到voice_id - 完整响应: %s", string(body))
	return "", ErrVoiceIDNotFound
}
