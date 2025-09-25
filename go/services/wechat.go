package services

import (
	"ai-celebrity-simulator/config"
	"ai-celebrity-simulator/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTP客户端配置常量
const (
	WeChatHTTPTimeout = 30 * time.Second
	WeChatMaxRetries  = 3
	WeChatRetryDelay  = 2 * time.Second
)

// WeChatGrantType 微信API配置常量
const (
	WeChatGrantType = "authorization_code"
)

// WeChatOAuthURL 微信API端点常量
const (
	WeChatOAuthURL = "https://api.weixin.qq.com/sns/jscode2session"
)

// WeChatStatusOK HTTP状态码常量
const (
	WeChatStatusOK = http.StatusOK
)

// 错误定义
var (
	ErrWeChatConfigMissing = errors.New("微信配置不完整")
	ErrWeChatRequestFailed = errors.New("微信API请求失败")
	ErrWeChatResponseError = errors.New("微信API响应错误")
	ErrWeChatParseError    = errors.New("微信API响应解析失败")
)

// WeChatResponse 微信小程序登录响应结构
type WeChatResponse struct {
	OpenID     string `json:"openid"`
	SessionKey string `json:"session_key"`
	UnionID    string `json:"unionid"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

// WeChatUserInfo 微信用户信息结构
type WeChatUserInfo struct {
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	Sex        int      `json:"sex"`
	Province   string   `json:"province"`
	City       string   `json:"city"`
	Country    string   `json:"country"`
	HeadImgURL string   `json:"headimgurl"`
	Privilege  []string `json:"privilege"`
	UnionID    string   `json:"unionid"`
	ErrCode    int      `json:"errcode"`
	ErrMsg     string   `json:"errmsg"`
}

// WeChatLogin 微信登录
func WeChatLogin(code string) (*WeChatResponse, error) {
	utils.LogInfo("开始微信登录 - Code长度: %d", len(code))

	// 验证输入参数
	if err := validateWeChatLoginParams(code); err != nil {
		utils.LogError("微信登录参数验证失败: %v", err)
		return nil, err
	}

	// 验证配置
	if err := validateWeChatConfig(); err != nil {
		utils.LogError("微信配置验证失败: %v", err)
		return nil, err
	}

	// 构建请求URL
	requestURL := buildWeChatLoginURL(code)

	// 发送请求
	response, err := sendWeChatLoginRequest(requestURL)
	if err != nil {
		utils.LogError("微信登录请求失败: %v", err)
		return nil, err
	}

	// 验证响应
	if err := validateWeChatResponse(response); err != nil {
		utils.LogError("微信登录响应验证失败: %v", err)
		return nil, err
	}

	utils.LogInfo("微信小程序登录成功 - OpenID长度: %d", len(response.OpenID))
	return response, nil
}

// validateWeChatLoginParams 验证微信登录参数
func validateWeChatLoginParams(code string) error {
	if code == "" {
		return errors.New("微信授权码不能为空")
	}
	return nil
}

// validateWeChatConfig 验证微信配置
func validateWeChatConfig() error {
	if config.AppConfig.WeChatAppID == "" || config.AppConfig.WeChatAppSecret == "" {
		return ErrWeChatConfigMissing
	}
	return nil
}

// buildWeChatLoginURL 构建微信小程序登录请求URL
func buildWeChatLoginURL(code string) string {
	params := url.Values{}
	params.Set("appid", config.AppConfig.WeChatAppID)
	params.Set("secret", config.AppConfig.WeChatAppSecret)
	params.Set("js_code", code)
	params.Set("grant_type", WeChatGrantType)

	return fmt.Sprintf("%s?%s", WeChatOAuthURL, params.Encode())
}

// sendWeChatLoginRequest 发送微信登录请求
func sendWeChatLoginRequest(requestURL string) (*WeChatResponse, error) {
	utils.LogInfo("发送微信登录请求 - URL: %s", maskSensitiveURL(requestURL))

	// 带重试的请求发送
	return sendWeChatRequestWithRetry(requestURL)
}

// maskSensitiveURL 脱敏URL中的敏感信息
func maskSensitiveURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "解析URL失败"
	}

	// 脱敏secret参数
	query := parsedURL.Query()
	if secret := query.Get("secret"); secret != "" && len(secret) > 8 {
		query.Set("secret", secret[:4]+"****"+secret[len(secret)-4:])
	}
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

// sendWeChatRequestWithRetry 带重试的微信请求发送
func sendWeChatRequestWithRetry(requestURL string) (*WeChatResponse, error) {
	var lastErr error

	for attempt := 1; attempt <= WeChatMaxRetries; attempt++ {
		utils.LogInfo("尝试微信登录请求 - 第 %d/%d 次", attempt, WeChatMaxRetries)

		response, err := sendSingleWeChatRequest(requestURL)
		if err == nil {
			utils.LogInfo("微信登录请求成功")
			return response, nil
		}

		lastErr = err
		utils.LogWarn("第 %d 次微信登录请求失败: %v", attempt, err)

		if attempt < WeChatMaxRetries {
			utils.LogInfo("等待 %v 后重试", WeChatRetryDelay)
			time.Sleep(WeChatRetryDelay)
		}
	}

	return nil, fmt.Errorf("微信登录请求失败，已重试 %d 次: %v", WeChatMaxRetries, lastErr)
}

// sendSingleWeChatRequest 发送单次微信请求
func sendSingleWeChatRequest(requestURL string) (*WeChatResponse, error) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), WeChatHTTPTimeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建微信登录请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "AI-Celebrity-Simulator/1.0")

	// 发送请求
	client := &http.Client{Timeout: WeChatHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("微信登录请求超时: %v", err)
		}
		return nil, fmt.Errorf("%w: %v", ErrWeChatRequestFailed, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("HTTP响应为空")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭响应体失败: %v", closeErr)
		}
	}()

	// 检查响应状态
	if resp.StatusCode != WeChatStatusOK {
		utils.LogError("微信API响应状态码错误 - 状态码: %d", resp.StatusCode)
		return nil, fmt.Errorf("%w: 状态码 %d", ErrWeChatResponseError, resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取微信API响应失败: %v", err)
	}

	utils.LogInfo("微信API响应 - 状态码: %d, 响应长度: %d", resp.StatusCode, len(body))

	// 解析响应
	return parseWeChatResponse(body)
}

// parseWeChatResponse 解析微信API响应
func parseWeChatResponse(body []byte) (*WeChatResponse, error) {
	var wechatResp WeChatResponse
	if err := json.Unmarshal(body, &wechatResp); err != nil {
		utils.LogError("微信API响应解析失败: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrWeChatParseError, err)
	}

	utils.LogInfo("微信API响应解析成功")
	return &wechatResp, nil
}

// validateWeChatResponse 验证微信API响应
func validateWeChatResponse(response *WeChatResponse) error {
	if response.ErrCode != 0 {
		return fmt.Errorf("微信API返回错误: %d - %s", response.ErrCode, response.ErrMsg)
	}

	if response.OpenID == "" {
		return errors.New("微信API响应中缺少OpenID")
	}

	return nil
}
