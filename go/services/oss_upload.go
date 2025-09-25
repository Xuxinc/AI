package services

import (
	"ai-celebrity-simulator/config"
	"ai-celebrity-simulator/utils"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSS配置常量
const (
	// OSSURLExpireTime URL签名过期时间（1年）
	OSSURLExpireTime   = 31536000 // 1年 = 31536000秒
	RandomStringLength = 8
	MaxUploadRetries   = 3
	OSSRetryDelay      = time.Second
)

// ValidImageExts 支持的图片格式常量
var (
	ValidImageExts = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
)

// 错误定义
var (
	ErrOSSConfigIncomplete = errors.New("OSS配置不完整")
	ErrOSSClientCreate     = errors.New("创建OSS客户端失败")
	ErrOSSBucketAccess     = errors.New("获取OSS bucket失败")
	ErrInvalidImageFormat  = errors.New("不支持的图片格式")
	ErrFileOpenFailed      = errors.New("打开文件失败")
	ErrFileUploadFailed    = errors.New("上传文件失败")
	ErrURLSignFailed       = errors.New("生成签名URL失败")
)

type OSSService struct {
	client *oss.Client
	bucket *oss.Bucket
}

var ossService *OSSService

// InitOSSService 初始化OSS服务
func InitOSSService() error {
	utils.LogInfo("开始初始化OSS服务")

	// 验证配置
	if err := validateOSSConfig(); err != nil {
		utils.LogError("OSS配置验证失败: %v", err)
		return err
	}

	// 创建OSS客户端
	client, err := createOSSClient()
	if err != nil {
		utils.LogError("创建OSS客户端失败: %v", err)
		return err
	}

	// 获取bucket
	bucket, err := getOSSBucket(client)
	if err != nil {
		utils.LogError("获取OSS bucket失败: %v", err)
		return err
	}

	ossService = &OSSService{
		client: client,
		bucket: bucket,
	}

	utils.LogInfo("OSS服务初始化成功")
	return nil
}

// validateOSSConfig 验证OSS配置
func validateOSSConfig() error {
	if config.AppConfig.OSSEndpoint == "" ||
		config.AppConfig.OSSAccessKeyID == "" ||
		config.AppConfig.OSSAccessKeySecret == "" ||
		config.AppConfig.OSSBucket == "" {
		return ErrOSSConfigIncomplete
	}
	return nil
}

// createOSSClient 创建OSS客户端
func createOSSClient() (*oss.Client, error) {
	// 确保endpoint使用HTTPS协议
	endpoint := config.AppConfig.OSSEndpoint
	if !strings.HasPrefix(endpoint, config.HTTPSProtocol) && !strings.HasPrefix(endpoint, config.HTTPProtocol) {
		endpoint = config.HTTPSProtocol + endpoint
	}

	utils.LogInfo("创建OSS客户端 - Endpoint: %s", endpoint)

	client, err := oss.New(endpoint, config.AppConfig.OSSAccessKeyID, config.AppConfig.OSSAccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOSSClientCreate, err)
	}

	return client, nil
}

// getOSSBucket 获取OSS bucket
func getOSSBucket(client *oss.Client) (*oss.Bucket, error) {
	utils.LogInfo("获取OSS bucket - Bucket: %s", config.AppConfig.OSSBucket)

	bucket, err := client.Bucket(config.AppConfig.OSSBucket)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOSSBucketAccess, err)
	}

	return bucket, nil
}

// UploadImages 上传多张图片
func (s *OSSService) UploadImages(files []*multipart.FileHeader) ([]string, error) {
	utils.LogInfo("开始批量上传图片 - 图片数量: %d", len(files))

	var urls []string
	var failedCount int

	for i, file := range files {
		utils.LogInfo("上传第 %d/%d 张图片 - 文件名: %s, 大小: %d bytes",
			i+1, len(files), file.Filename, file.Size)

		url, err := s.uploadSingleImage(file)
		if err != nil {
			utils.LogError("上传第 %d 张图片失败 - 文件名: %s, 错误: %v",
				i+1, file.Filename, err)
			failedCount++
			continue
		}

		urls = append(urls, url)
		utils.LogInfo("第 %d 张图片上传成功 - URL长度: %d", i+1, len(url))
	}

	if failedCount > 0 {
		utils.LogWarn("批量上传完成 - 成功: %d, 失败: %d", len(urls), failedCount)
	} else {
		utils.LogInfo("批量上传全部成功 - 共 %d 张图片", len(urls))
	}

	if len(urls) == 0 {
		return nil, errors.New("所有图片上传失败")
	}

	return urls, nil
}

// uploadSingleImage 上传单张图片
func (s *OSSService) uploadSingleImage(file *multipart.FileHeader) (string, error) {
	// 检查文件类型
	if err := validateImageFile(file); err != nil {
		return "", err
	}

	// 生成唯一文件名
	objectKey := generateObjectKey(file.Filename)

	// 上传文件
	url, err := s.uploadFileWithRetry(file, objectKey)
	if err != nil {
		return "", err
	}

	return url, nil
}

// validateImageFile 验证图片文件
func validateImageFile(file *multipart.FileHeader) error {
	if !isValidImageType(file.Filename) {
		return fmt.Errorf("%w: %s", ErrInvalidImageFormat, file.Filename)
	}
	return nil
}

// uploadFileWithRetry 带重试的文件上传
func (s *OSSService) uploadFileWithRetry(file *multipart.FileHeader, objectKey string) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= MaxUploadRetries; attempt++ {
		utils.LogInfo("尝试上传文件 - 第 %d/%d 次, ObjectKey: %s",
			attempt, MaxUploadRetries, objectKey)

		url, err := s.uploadFile(file, objectKey)
		if err == nil {
			utils.LogInfo("文件上传成功 - ObjectKey: %s", objectKey)
			return url, nil
		}

		lastErr = err
		utils.LogWarn("第 %d 次上传失败 - ObjectKey: %s, 错误: %v",
			attempt, objectKey, err)

		if attempt < MaxUploadRetries {
			utils.LogInfo("等待 %v 后重试", OSSRetryDelay)
			time.Sleep(OSSRetryDelay)
		}
	}

	return "", fmt.Errorf("上传失败，已重试 %d 次: %v", MaxUploadRetries, lastErr)
}

// uploadFile 上传文件
func (s *OSSService) uploadFile(file *multipart.FileHeader, objectKey string) (string, error) {
	// 打开文件
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileOpenFailed, err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			utils.LogWarn("⚠️ 关闭文件失败: %v", closeErr)
		}
	}()

	// 上传到OSS
	err = s.bucket.PutObject(objectKey, src)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrFileUploadFailed, err)
	}

	// 生成签名URL
	signedURL, err := s.generateSignedURL(objectKey)
	if err != nil {
		return "", err
	}

	return signedURL, nil
}

// generateSignedURL 生成签名URL
func (s *OSSService) generateSignedURL(objectKey string) (string, error) {
	// 生成带签名的永久访问URL（设置为1年过期）
	signedURL, err := s.bucket.SignURL(objectKey, oss.HTTPGet, OSSURLExpireTime)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrURLSignFailed, err)
	}

	// 确保返回HTTPS协议的URL
	signedURL = strings.Replace(signedURL, config.HTTPProtocol, config.HTTPSProtocol, 1)

	return signedURL, nil
}

// GetOSSService 获取OSS服务实例
func GetOSSService() *OSSService {
	return ossService
}

// isValidImageType 检查是否为有效的图片类型
func isValidImageType(filename string) bool {
	if filename == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filename))

	for _, validExt := range ValidImageExts {
		if ext == validExt {
			return true
		}
	}

	return false
}

// generateObjectKey 生成唯一的对象键
func generateObjectKey(filename string) string {
	ext := filepath.Ext(filename)
	timestamp := time.Now().Unix()
	randomStr := generateRandomString(RandomStringLength)
	return fmt.Sprintf("xxc/images/%d_%s%s", timestamp, randomStr, ext)
}

// generateRandomString 生成随机字符串
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if length <= 0 {
		return ""
	}

	b := make([]byte, length)
	for i := range b {
		// 使用时间戳和索引作为随机种子
		b[i] = charset[int(time.Now().UnixNano()+int64(i))%len(charset)]
	}
	return string(b)
}
