package services

import (
	"ai-celebrity-simulator/config"
	"ai-celebrity-simulator/utils"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/google/uuid"
)

// HTTP客户端配置常量
const (
	CharacterHTTPTimeout = 30 * time.Second
	MaxRetryAttempts     = 3
)

// AI模型配置常量
const (
	CharacterModel       = "qwen-plus"
	ImageModel           = "wanx2.1-t2i-turbo"
	CharacterMaxTokens   = 1000
	CharacterTemperature = 0.7
	CharacterTopP        = 0.9
)

// 图片生成配置常量
const (
	ImageSize    = "1024*1024"
	ImageCount   = 1
	OssUrlExpire = 31536000 // 1年
)

// 字符串操作常量
const (
	// StringReplaceCount 字符串替换操作
	StringReplaceCount = 1

	// StringIndexEnd 字符串索引操作
	StringIndexEnd = 1
)

// OSS路径和API配置常量
const (
	// OssBasePath OSS存储路径
	OssBasePath      = "xxc"
	OssCelebrityPath = "xxc/celebrity-avatars"

	// TimeFormatFilename 时间格式
	TimeFormatFilename = "20060102150405"

	// RetryDelay1 重试延迟时间
	RetryDelay1 = 3 * time.Second
	RetryDelay2 = 7 * time.Second
	RetryDelay3 = 5 * time.Second

	// DashScopeImageAPI 外部API端点
	DashScopeImageAPI = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis"
	DashScopeTaskAPI  = "https://dashscope.aliyuncs.com/api/v1/tasks"
)

// 音色配置常量
const (
	MaleVoiceModel   = "longtian_v2"
	FemaleVoiceModel = "longxiaochun_v2"
)

// AI提示词模板常量
const (
	// CelebrityPromptTemplate 名人角色生成提示词模板
	CelebrityPromptTemplate = `请为历史名人"%s"生成完整的角色信息。

要求：
1. 简短的人物描述（10-30字）
2. 角色扮演提示词（40-100字）
3. 根据角色性别选择合适的音色：
   - 男性角色使用：%s
   - 女性角色使用：%s

请直接返回JSON格式，不要包含任何其他文字或说明：

{
  "description": "人物描述",
  "prompt": "角色扮演提示词", 
  "voice_model": "音色名称"
}`

	// CustomPromptTemplate 自定义角色生成提示词模板
	CustomPromptTemplate = `根据用户提供的信息，为自定义角色"%s"生成完整的角色信息。

用户描述：%s

要求：
1. 简短的人物描述（10-30字）
2. 角色扮演提示词（40-100字）
3. 根据角色性别选择合适的音色：
   - 男性角色使用：%s
   - 女性角色使用：%s

请直接返回JSON格式，不要包含任何其他文字或说明：

{
  "description": "人物描述",
  "prompt": "角色扮演提示词",
  "voice_model": "音色名称"
}`

	// CharacterSystemPromptTemplate 系统提示词模板
	CharacterSystemPromptTemplate = `你是一个专业的角色设计师。你的任务是为AI聊天机器人设计角色设定。重要：请只返回JSON格式，不要包含任何其他文字、说明或解释。JSON必须包含description、prompt、voice_model三个字段。音色选择规则：男性角色使用%s，女性角色使用%s。`

	// AvatarPromptTemplate 头像生成提示词模板
	AvatarPromptTemplate = `生成一张符合%s名人身份的高清真人头像，背景简洁，风格真实、清晰，适合社交媒体头像。要求如下：
- 头像应为正面或侧面视角，面部表情自然。
- 图像分辨率至少为1024x1024像素，确保图像清晰。
- 背景应为纯色或极简风格，避免复杂元素干扰。
- 人物应穿着符合其时代和身份的服装。
- 图像风格应适合社交媒体使用，避免过于夸张或不真实的元素。
- 请确保图像符合真实人物的外貌特征，避免过度艺术化。`
)

// CharacterStatusOK HTTP状态码常量
const (
	CharacterStatusOK = http.StatusOK
)

// 错误定义
var (
	ErrCharacterInvalidResponse = errors.New("无效的响应格式")
	ErrCharacterEmptyResponse   = errors.New("响应为空")
	ErrAPICallFailed            = errors.New("API调用失败")
	ErrMissingFields            = errors.New("缺少必要字段")
)

// CharacterGenerationRequest 角色生成请求结构
type CharacterGenerationRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// createOSSBucket 创建OSS Bucket的辅助函数
func createOSSBucket() (*oss.Bucket, error) {
	// 确保endpoint使用HTTPS协议
	endpoint := config.AppConfig.OSSEndpoint
	if !strings.HasPrefix(endpoint, config.HTTPSProtocol) && !strings.HasPrefix(endpoint, config.HTTPProtocol) {
		endpoint = config.HTTPSProtocol + endpoint
	}

	client, err := oss.New(endpoint, config.AppConfig.OSSAccessKeyID, config.AppConfig.OSSAccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建OSS客户端失败: %v", err)
	}

	bucket, err := client.Bucket(config.AppConfig.OSSBucket)
	if err != nil {
		return nil, fmt.Errorf("获取Bucket失败: %v", err)
	}

	return bucket, nil
}

// CharacterGenerationResponse 角色生成响应结构
type CharacterGenerationResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	VoiceModel  string `json:"voice_model"`
	AvatarURL   string `json:"avatar_url"`
}

// ImageAsyncRequest 图片生成相关结构
type ImageAsyncRequest struct {
	Model      string          `json:"model"`
	Input      ImageInput      `json:"input"`
	Parameters ImageParameters `json:"parameters"`
}

type ImageInput struct {
	Prompt string `json:"prompt"`
}

type ImageParameters struct {
	Size string `json:"size"`
	N    int    `json:"n"`
}

type ImageTaskResponse struct {
	Output struct {
		TaskStatus string `json:"task_status"`
		TaskID     string `json:"task_id"`
	} `json:"output"`
	RequestID string `json:"request_id"`
}

type ImageTaskResult struct {
	Output struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
		TaskStatus string `json:"task_status"`
	} `json:"output"`
}

// GenerateCelebrityCharacter 生成名人角色信息
func GenerateCelebrityCharacter(name string) (*CharacterGenerationResponse, error) {
	utils.LogInfo("开始生成名人角色 - 名称: %s", name)

	// 验证输入参数
	if err := validateCharacterName(name); err != nil {
		utils.LogError("名人角色参数验证失败 - 名称: %s, 错误: %v", name, err)
		return nil, err
	}

	// 生成角色信息
	response, err := generateCelebrityInfo(name)
	if err != nil {
		utils.LogError("生成名人角色信息失败 - 名称: %s, 错误: %v", name, err)
		return nil, err
	}

	// 生成头像
	avatarURL, err := generateAndUploadAvatar(name)
	if err != nil {
		utils.LogError("生成名人头像失败 - 名称: %s, 错误: %v", name, err)
		return nil, err
	}

	response.Name = name
	response.AvatarURL = avatarURL

	utils.LogInfo("名人角色生成成功 - 名称: %s, 头像URL长度: %d", name, len(avatarURL))
	return response, nil
}

// GenerateCustomCharacter 生成自定义角色信息
func GenerateCustomCharacter(name, description, avatarURL string) (*CharacterGenerationResponse, error) {
	utils.LogInfo("开始生成自定义角色 - 名称: %s, 描述长度: %d", name, len(description))

	// 验证输入参数
	if err := validateCustomCharacterParams(name, description); err != nil {
		utils.LogError("自定义角色参数验证失败 - 名称: %s, 错误: %v", name, err)
		return nil, err
	}

	// 生成角色信息
	response, err := generateCustomInfo(name, description)
	if err != nil {
		utils.LogError("生成自定义角色信息失败 - 名称: %s, 错误: %v", name, err)
		return nil, err
	}

	response.Name = name
	response.AvatarURL = avatarURL // 直接用前端上传的url

	utils.LogInfo("自定义角色生成成功 - 名称: %s", name)
	return response, nil
}

// validateCharacterName 验证角色名称
func validateCharacterName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("角色名称不能为空")
	}
	return nil
}

// validateCustomCharacterParams 验证自定义角色参数
func validateCustomCharacterParams(name, description string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("角色名称不能为空")
	}
	if strings.TrimSpace(description) == "" {
		return errors.New("角色描述不能为空")
	}
	return nil
}

// generateCelebrityInfo 生成名人信息
func generateCelebrityInfo(name string) (*CharacterGenerationResponse, error) {
	prompt := buildCelebrityPrompt(name)
	return callAIForCharacterGeneration(prompt)
}

// generateCustomInfo 生成自定义信息
func generateCustomInfo(name, description string) (*CharacterGenerationResponse, error) {
	prompt := buildCustomPrompt(name, description)
	return callAIForCharacterGeneration(prompt)
}

// buildCelebrityPrompt 构建名人提示词
func buildCelebrityPrompt(name string) string {
	return fmt.Sprintf(CelebrityPromptTemplate, name, MaleVoiceModel, FemaleVoiceModel)
}

// buildCustomPrompt 构建自定义提示词
func buildCustomPrompt(name, description string) string {
	return fmt.Sprintf(CustomPromptTemplate, name, description, MaleVoiceModel, FemaleVoiceModel)
}

// callAIForCharacterGeneration 调用AI生成角色信息
func callAIForCharacterGeneration(prompt string) (*CharacterGenerationResponse, error) {
	utils.LogInfo("开始调用AI生成角色信息")

	// 构建请求
	request, err := buildCharacterRequest(prompt)
	if err != nil {
		return nil, fmt.Errorf("构建AI请求失败: %v", err)
	}

	// 发送请求
	response, err := sendCharacterRequest(request)
	if err != nil {
		return nil, fmt.Errorf("发送AI请求失败: %v", err)
	}

	// 解析响应
	result, err := parseCharacterResponse(response)
	if err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %v", err)
	}

	utils.LogInfo("AI角色信息生成成功")
	return result, nil
}

// buildCharacterRequest 构建角色生成请求
func buildCharacterRequest(prompt string) ([]byte, error) {
	messages := []map[string]string{
		{
			"role":    "system",
			"content": buildSystemPrompt(),
		},
		{
			"role":    "user",
			"content": prompt,
		},
	}

	requestBody := map[string]interface{}{
		"model":       CharacterModel,
		"messages":    messages,
		"max_tokens":  CharacterMaxTokens,
		"temperature": CharacterTemperature,
		"top_p":       CharacterTopP,
	}

	return json.Marshal(requestBody)
}

// buildSystemPrompt 构建系统提示词
func buildSystemPrompt() string {
	return fmt.Sprintf(CharacterSystemPromptTemplate, MaleVoiceModel, FemaleVoiceModel)
}

// sendCharacterRequest 发送角色生成请求
func sendCharacterRequest(jsonData []byte) ([]byte, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), CharacterHTTPTimeout)
	defer cancel()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", config.AppConfig.BailianBaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.BailianAPIKey)

	// 发送请求
	client := &http.Client{Timeout: CharacterHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("请求超时: %v", err)
		}
		return nil, fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	// 检查状态码
	if resp.StatusCode != CharacterStatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		utils.LogError("AI API响应错误 - 状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
		return nil, ErrAPICallFailed
	}

	// 读取响应体
	return io.ReadAll(resp.Body)
}

// parseCharacterResponse 解析角色生成响应
func parseCharacterResponse(body []byte) (*CharacterGenerationResponse, error) {
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error map[string]interface{} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查API错误
	if apiResponse.Error != nil {
		utils.LogError("AI API返回错误: %+v", apiResponse.Error)
		return nil, ErrAPICallFailed
	}

	if len(apiResponse.Choices) == 0 {
		return nil, ErrCharacterEmptyResponse
	}

	content := apiResponse.Choices[0].Message.Content
	utils.LogInfo("AI原始响应长度: %d", len(content))

	// 提取JSON部分
	jsonContent := extractJSONFromContent(content)
	if jsonContent == "" {
		utils.LogError("无法从AI响应中提取JSON内容")
		return nil, ErrCharacterInvalidResponse
	}

	utils.LogInfo("提取的JSON内容长度: %d", len(jsonContent))

	// 解析JSON响应
	var result CharacterGenerationResponse
	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		utils.LogError("解析AI响应JSON失败: %v, JSON内容: %s", err, jsonContent)
		return nil, fmt.Errorf("解析AI响应JSON失败: %v", err)
	}

	// 验证必要字段
	if err := validateCharacterResponse(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// validateCharacterResponse 验证角色响应
func validateCharacterResponse(result *CharacterGenerationResponse) error {
	if result.Description == "" || result.Prompt == "" || result.VoiceModel == "" {
		utils.LogError("AI响应缺少必要字段 - description: %s, prompt: %s, voice_model: %s",
			result.Description, result.Prompt, result.VoiceModel)
		return ErrMissingFields
	}
	return nil
}

// generateAndUploadAvatar 生成并上传头像
func generateAndUploadAvatar(name string) (string, error) {
	utils.LogInfo("开始生成头像 - 名称: %s", name)

	// 生成头像
	imageURL, err := generateAvatarImage(name)
	if err != nil {
		return "", fmt.Errorf("生成头像失败: %v", err)
	}

	// 下载并上传到OSS
	ossURL, err := downloadAndUploadToOSS(imageURL, name)
	if err != nil {
		return "", fmt.Errorf("上传头像失败: %v", err)
	}

	utils.LogInfo("头像生成并上传成功 - 名称: %s", name)
	return ossURL, nil
}

// generateAvatarImage 生成头像图片
func generateAvatarImage(name string) (string, error) {
	prompt := buildAvatarPrompt(name)

	// 创建异步任务
	taskID, err := createImageTask(prompt)
	if err != nil {
		return "", fmt.Errorf("创建图片生成任务失败: %v", err)
	}

	// 获取任务结果
	imageURL, err := waitForImageTask(taskID)
	if err != nil {
		return "", fmt.Errorf("获取图片生成结果失败: %v", err)
	}

	return imageURL, nil
}

// buildAvatarPrompt 构建头像提示词
func buildAvatarPrompt(name string) string {
	return fmt.Sprintf(AvatarPromptTemplate, name)
}

// createImageTask 创建图片生成任务
func createImageTask(prompt string) (string, error) {
	requestBody := ImageAsyncRequest{
		Model: ImageModel,
		Input: ImageInput{
			Prompt: prompt,
		},
		Parameters: ImageParameters{
			Size: ImageSize,
			N:    ImageCount,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %v", err)
	}

	req, err := http.NewRequest(
		"POST",
		DashScopeImageAPI,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.BailianAPIKey)
	req.Header.Set("X-DashScope-Async", "enable")

	client := &http.Client{Timeout: CharacterHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求发送失败: %v", err)
	}
	if resp == nil {
		return "", fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	if resp.StatusCode != CharacterStatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API错误(%d): %s", resp.StatusCode, string(bodyBytes))
	}

	var taskResp ImageTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	return taskResp.Output.TaskID, nil
}

// waitForImageTask 获取图片生成任务结果
func waitForImageTask(taskID string) (string, error) {
	url := fmt.Sprintf("%s/%s", DashScopeTaskAPI, taskID)

	delays := []time.Duration{
		RetryDelay1,
		RetryDelay2,
		RetryDelay3,
	}

	for i := 0; i < MaxRetryAttempts; i++ {
		result, err := checkImageTaskStatus(url)
		if err != nil {
			if i < len(delays) {
				time.Sleep(delays[i])
				continue
			}
			return "", fmt.Errorf("超过最大重试次数: %v", err)
		}

		switch result.Output.TaskStatus {
		case "SUCCEEDED":
			if len(result.Output.Results) > 0 {
				return result.Output.Results[0].URL, nil
			}
			return "", fmt.Errorf("任务成功但无结果")
		case "FAILED":
			return "", fmt.Errorf("任务处理失败")
		case "PENDING", "RUNNING":
			if i < len(delays) {
				time.Sleep(delays[i])
				continue
			}
			return "", fmt.Errorf("超过最大重试次数")
		}
	}

	return "", fmt.Errorf("超过最大重试次数")
}

// downloadImage 下载图片
func downloadImage(url, name string) (string, func(), error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", nil, fmt.Errorf("下载失败: %v", err)
	}
	if resp == nil {
		return "", nil, fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	tempDir := os.TempDir()
	filename := fmt.Sprintf("%s_%s.jpg", uuid.New().String(), name)
	filePath := filepath.Join(tempDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("创建文件失败: %v", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭文件失败: %v", closeErr)
		}
		if removeErr := os.Remove(filePath); removeErr != nil {
			utils.LogWarn("⚠️ 删除临时文件失败: %v", removeErr)
		}
		return "", nil, fmt.Errorf("保存文件失败: %v", err)
	}
	if closeErr := file.Close(); closeErr != nil {
		utils.LogWarn("⚠️ 关闭文件失败: %v", closeErr)
	}

	cleanup := func() {
		if err := os.Remove(filePath); err != nil {
			utils.LogError("删除临时文件失败: %v", err)
		}
	}

	return filePath, cleanup, nil
}

// downloadAndUploadToOSS 下载并上传到OSS
func downloadAndUploadToOSS(imageURL, name string) (string, error) {
	// 下载图片
	localPath, cleanup, err := downloadImage(imageURL, name)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %v", err)
	}
	defer cleanup()

	// 上传到OSS
	ossURL, err := uploadToOSS(localPath)
	if err != nil {
		return "", fmt.Errorf("上传图片到OSS失败: %v", err)
	}

	return ossURL, nil
}

// uploadToOSS 上传文件到OSS
func uploadToOSS(localPath string) (string, error) {
	bucket, err := createOSSBucket()
	if err != nil {
		return "", err
	}

	objectName := OssCelebrityPath + "/" + filepath.Base(localPath)
	if err := bucket.PutObjectFromFile(objectName, localPath); err != nil {
		return "", fmt.Errorf("上传文件失败: %v", err)
	}

	// 生成带签名的永久访问URL（设置为1年过期）
	signedURL, err := bucket.SignURL(objectName, oss.HTTPGet, OssUrlExpire) // 1年 = 31536000秒
	if err != nil {
		return "", fmt.Errorf("生成签名URL失败: %v", err)
	}

	// 确保返回HTTPS协议的URL
	signedURL = strings.Replace(signedURL, config.HTTPProtocol, config.HTTPSProtocol, StringReplaceCount)

	return signedURL, nil
}

// 从AI响应中提取JSON内容
func extractJSONFromContent(content string) string {
	// 查找第一个 { 和最后一个 }
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start == -1 || end == -1 || start >= end {
		return ""
	}

	return content[start : end+StringIndexEnd]
}

// UploadFileToOSS 上传文件到OSS
func UploadFileToOSS(file interface{}, folder string) (string, error) {
	// 只支持 *multipart.FileHeader
	fileHeader, ok := file.(*multipart.FileHeader)
	if !ok {
		return "", fmt.Errorf("文件类型错误")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭文件失败: %v", closeErr)
		}
	}()

	bucket, err := createOSSBucket()
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("%s/%s/%s_%s%s", OssBasePath, folder, uuid.New().String(), time.Now().Format(TimeFormatFilename), filepath.Ext(fileHeader.Filename))
	// 上传
	if err := bucket.PutObject(filename, f); err != nil {
		return "", fmt.Errorf("上传文件失败: %v", err)
	}

	// 生成带签名的永久访问URL（设置为1年过期）
	signedURL, err := bucket.SignURL(filename, oss.HTTPGet, OssUrlExpire) // 1年 = 31536000秒
	if err != nil {
		return "", fmt.Errorf("生成签名URL失败: %v", err)
	}

	// 确保返回HTTPS协议的URL
	signedURL = strings.Replace(signedURL, config.HTTPProtocol, config.HTTPSProtocol, StringReplaceCount)

	return signedURL, nil
}

// checkImageTaskStatus 检查图片生成任务状态
func checkImageTaskStatus(url string) (*ImageTaskResult, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.AppConfig.BailianAPIKey)

	client := &http.Client{Timeout: CharacterHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求发送失败: %v", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	if resp.StatusCode != CharacterStatusOK {
		return nil, fmt.Errorf("API错误(%d)", resp.StatusCode)
	}

	var result ImageTaskResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &result, nil
}
