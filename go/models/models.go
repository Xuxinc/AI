package models

import (
	"time"
)

// User 用户表
type User struct {
	ID        uint      `json:"id" gorm:"primaryKey;type:bigint unsigned"`
	OpenID    string    `json:"openid" gorm:"column:openid;uniqueIndex;size:64"`
	Nickname  string    `json:"nickname" gorm:"size:64"`
	Avatar    string    `json:"avatar" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Character 名人角色表
type Character struct {
	ID              uint      `json:"id" gorm:"primaryKey;type:bigint unsigned"`
	Name            string    `json:"name" gorm:"size:64"`
	Description     string    `json:"description" gorm:"type:text"`
	Prompt          string    `json:"prompt" gorm:"type:text"`
	VoiceModel      string    `json:"voice_model" gorm:"column:voice_model;size:128"`
	AvatarURL       string    `json:"avatar_url" gorm:"column:avatar_url;type:text"`
	IsCreatedByUser string    `json:"is_created_by_user" gorm:"column:is_created_by_user;size:3;default:'no'"`
	UID             *uint     `json:"uid" gorm:"column:uid"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Dialog 会话表
type Dialog struct {
	ID          uint      `json:"id" gorm:"primaryKey;type:bigint unsigned"`
	UserID      uint      `json:"user_id" gorm:"column:user_id;uniqueIndex:idx_user_character"`
	CharacterID uint      `json:"character_id" gorm:"column:character_id;uniqueIndex:idx_user_character"`
	IsTop       string    `json:"is_top" gorm:"column:is_top;size:3;default:'no'"` // yes 或 no
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`

	User      User      `json:"user" gorm:"foreignKey:UserID"`
	Character Character `json:"character" gorm:"foreignKey:CharacterID"`
	Messages  []Message `json:"messages" gorm:"foreignKey:DialogID"`
}

// TableName 指定表名
func (Dialog) TableName() string {
	return "dialogs"
}

// Message 会话消息表
type Message struct {
	ID         uint      `json:"id" gorm:"primaryKey;type:bigint unsigned"`
	DialogID   uint      `json:"dialog_id" gorm:"column:dialog_id"`
	Content    string    `json:"content" gorm:"column:content;type:text"`
	IsVoice    string    `json:"is_voice" gorm:"column:is_voice;size:3;default:'no'"` // yes 或 no
	VoiceURL   string    `json:"voice_url" gorm:"column:voice_url;type:text"`
	PictureURL string    `json:"picture_url" gorm:"column:picture_url;type:text"`         // 新增：图片URL字段
	IsDeleted  string    `json:"is_deleted" gorm:"column:is_deleted;size:3;default:'no'"` // yes 或 no
	Role       string    `json:"role" gorm:"column:role;size:10"`                         // user 或 ai
	Time       time.Time `json:"time" gorm:"column:time"`

	Dialog Dialog `json:"dialog" gorm:"foreignKey:DialogID"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
