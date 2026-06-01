// internal/models/models.go
package models

import "time"

type APIKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Key        string     `gorm:"uniqueIndex;not null" json:"-"`
	Alias      string     `gorm:"not null;default:''" json:"alias"`
	IsActive   bool       `gorm:"not null;default:true" json:"is_active"`
	IsInvalid  bool       `gorm:"not null;default:false" json:"is_invalid"`
	CooldownAt *time.Time `json:"cooldown_at"`
	UsedCount  int64      `gorm:"not null;default:0" json:"used_count"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type RequestLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RequestID  string    `gorm:"index;not null" json:"request_id"`
	KeyID      uint      `gorm:"column:key_id;index" json:"key_id"`
	KeyAlias   string    `json:"key_alias"`
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
