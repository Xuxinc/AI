package utils

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 雪花算法配置常量
const (
	// MachineIDBits 机器ID位数
	MachineIDBits = 10
	// SequenceBits 序列号位数
	SequenceBits = 12

	// MaxMachineID 最大值
	MaxMachineID = (1 << MachineIDBits) - 1
	MaxSequence  = (1 << SequenceBits) - 1

	// MachineIDShift 偏移量
	MachineIDShift = SequenceBits
	TimestampShift = SequenceBits + MachineIDBits

	// EpochTimestamp 起始时间戳 (2023-01-01 00:00:00 UTC)
	EpochTimestamp = int64(1672531200000)

	// DefaultMachineID 默认机器ID
	DefaultMachineID = 1

	// SnowflakeTokenPrefix Token前缀
	SnowflakeTokenPrefix = "token_"

	// ClockBackwardTolerance 时钟回退容忍时间（毫秒）
	ClockBackwardTolerance = 5
)

// 错误定义
var (
	ErrMachineIDOutOfRange = errors.New("机器ID超出范围")
	ErrClockBackward       = errors.New("系统时钟回退")
	ErrSnowflakeNotInit    = errors.New("雪花算法实例未初始化")
	ErrIDGenerationFailed  = errors.New("ID生成失败")
)

// Snowflake 雪花算法结构
type Snowflake struct {
	mutex     sync.Mutex
	machineID int64
	sequence  int64
	lastTime  int64
}

var (
	// 全局雪花算法实例
	globalSnowflake *Snowflake
	once            sync.Once
	initError       error
)

// NewSnowflake 创建新的雪花算法实例
func NewSnowflake(machineID int64) (*Snowflake, error) {
	if machineID < 0 || machineID > MaxMachineID {
		LogError("机器ID超出范围 - 机器ID: %d, 有效范围: 0-%d", machineID, MaxMachineID)
		return nil, fmt.Errorf("%w: 机器ID %d, 有效范围: 0-%d", ErrMachineIDOutOfRange, machineID, MaxMachineID)
	}

	LogInfo("创建雪花算法实例 - 机器ID: %d", machineID)

	return &Snowflake{
		machineID: machineID,
		sequence:  0,
		lastTime:  0,
	}, nil
}

// GetSnowflake 获取全局雪花算法实例
func GetSnowflake() (*Snowflake, error) {
	once.Do(func() {
		LogInfo("初始化全局雪花算法实例")
		globalSnowflake, initError = NewSnowflake(DefaultMachineID)
		if initError != nil {
			LogError("全局雪花算法实例初始化失败: %v", initError)
		} else {
			LogInfo("全局雪花算法实例初始化成功")
		}
	})

	if initError != nil {
		return nil, initError
	}

	if globalSnowflake == nil {
		LogError("全局雪花算法实例为nil")
		return nil, ErrSnowflakeNotInit
	}

	return globalSnowflake, nil
}

// NextID 生成下一个ID
func (s *Snowflake) NextID() (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now().UnixMilli()

	// 检查时钟回退
	if now < s.lastTime {
		clockBackward := s.lastTime - now
		LogWarn("检测到系统时钟回退 - 回退时间: %d毫秒", clockBackward)

		// 如果回退时间在容忍范围内，等待追上
		if clockBackward <= ClockBackwardTolerance {
			LogInfo("时钟回退在容忍范围内，等待追上 - 等待时间: %d毫秒", clockBackward)
			time.Sleep(time.Duration(clockBackward) * time.Millisecond)
			now = time.Now().UnixMilli()
		} else {
			LogError("系统时钟回退超出容忍范围 - 回退时间: %d毫秒, 容忍范围: %d毫秒",
				clockBackward, ClockBackwardTolerance)
			return 0, fmt.Errorf("%w: 回退 %d 毫秒", ErrClockBackward, clockBackward)
		}
	}

	// 如果是同一毫秒内，序列号递增
	if now == s.lastTime {
		s.sequence = (s.sequence + 1) & MaxSequence
		// 如果序列号溢出，等待下一毫秒
		if s.sequence == 0 {
			LogDebug("序列号溢出，等待下一毫秒")
			now = s.waitNextMillis(s.lastTime)
		}
	} else {
		// 不同毫秒，序列号重置为0
		s.sequence = 0
	}

	s.lastTime = now

	// 生成ID
	id := ((now - EpochTimestamp) << TimestampShift) |
		(s.machineID << MachineIDShift) |
		s.sequence

	if id <= 0 {
		LogError("生成的ID无效 - ID: %d", id)
		return 0, ErrIDGenerationFailed
	}

	LogDebug("生成ID成功 - ID: %d, 时间戳: %d, 机器ID: %d, 序列号: %d",
		id, now-EpochTimestamp, s.machineID, s.sequence)

	return id, nil
}

// waitNextMillis 等待下一毫秒
func (s *Snowflake) waitNextMillis(lastTimestamp int64) int64 {
	timestamp := time.Now().UnixMilli()
	waitCount := 0
	for timestamp <= lastTimestamp {
		waitCount++
		if waitCount > 1000 { // 防止无限循环
			LogWarn("等待下一毫秒次数过多 - 等待次数: %d", waitCount)
		}
		time.Sleep(time.Microsecond * 100) // 减少CPU占用
		timestamp = time.Now().UnixMilli()
	}

	if waitCount > 10 {
		LogDebug("等待下一毫秒完成 - 等待次数: %d", waitCount)
	}

	return timestamp
}

// GenerateToken 生成token
func GenerateToken() (string, error) {
	snowflake, err := GetSnowflake()
	if err != nil {
		LogError("获取雪花算法实例失败: %v", err)
		return "", fmt.Errorf("获取雪花算法实例失败: %w", err)
	}

	id, err := snowflake.NextID()
	if err != nil {
		LogError("生成ID失败: %v", err)
		return "", fmt.Errorf("生成ID失败: %w", err)
	}

	token := fmt.Sprintf("%s%d", SnowflakeTokenPrefix, id)
	LogDebug("生成Token成功 - Token长度: %d", len(token))

	return token, nil
}

// GenerateUserID 生成用户ID
func GenerateUserID() (uint, error) {
	snowflake, err := GetSnowflake()
	if err != nil {
		LogError("获取雪花算法实例失败: %v", err)
		return 0, fmt.Errorf("获取雪花算法实例失败: %w", err)
	}

	id, err := snowflake.NextID()
	if err != nil {
		LogError("生成用户ID失败: %v", err)
		return 0, fmt.Errorf("生成用户ID失败: %w", err)
	}

	// 确保ID为正数且在uint范围内
	if id <= 0 {
		LogError("生成的用户ID无效 - ID: %d", id)
		return 0, ErrIDGenerationFailed
	}

	userID := uint(id)
	LogDebug("生成用户ID成功 - 用户ID: %d", userID)

	return userID, nil
}

// GetMachineID 获取机器ID
func (s *Snowflake) GetMachineID() int64 {
	return s.machineID
}

// GetSequence 获取当前序列号
func (s *Snowflake) GetSequence() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.sequence
}

// GetLastTimestamp 获取上次生成时间戳
func (s *Snowflake) GetLastTimestamp() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.lastTime
}
