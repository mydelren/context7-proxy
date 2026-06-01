// internal/models/models.go
package models

import (
	"time"
)

type Admin struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string `gorm:"not null" json:"-"`
}

type Member struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Token       string    `gorm:"uniqueIndex;not null" json:"-"`
	TokenSuffix string    `gorm:"-" json:"token_suffix"`
	APIKeyID    *uint     `gorm:"index" json:"api_key_id"`
	Strategy    string    `gorm:"not null;default:''" json:"strategy"`
	IsActive    bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type APIKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Key        string     `gorm:"uniqueIndex;not null" json:"-"`
	Alias      string     `gorm:"not null;default:''" json:"alias"`
	IsActive   bool       `gorm:"not null;default:true" json:"is_active"`
	IsInvalid  bool       `gorm:"not null;default:false" json:"is_invalid"`
	CooldownAt *time.Time `json:"cooldown_at"`
	UsedCount   int64      `gorm:"not null;default:0" json:"used_count"`
	MaxRequests int64      `gorm:"not null;default:1000" json:"max_requests"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type RequestLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RequestID  string    `gorm:"index;not null" json:"request_id"`
	KeyID      uint      `gorm:"column:key_id;index" json:"key_id"`
	KeyAlias   string    `json:"key_alias"`
	MemberID   uint      `gorm:"index" json:"member_id"`
	MemberName string    `json:"member_name"`
	Method     string    `gorm:"not null;default:''" json:"method"`
	Endpoint   string    `gorm:"index;not null" json:"endpoint"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	ClientIP   string    `json:"client_ip"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

type MasterKey struct {
	ID  uint   `gorm:"primaryKey"`
	Key string `gorm:"uniqueIndex;not null"`
}

type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `gorm:"not null;default:''" json:"value"`
}
