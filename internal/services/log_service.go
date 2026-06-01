// internal/services/log_service.go
package services

import (
	"context"
	"log"
	"time"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type LogService struct{ db *gorm.DB }

func NewLogService(db *gorm.DB) *LogService { return &LogService{db: db} }

func (s *LogService) Create(ctx context.Context, l *models.RequestLog) error {
	return s.db.WithContext(ctx).Create(l).Error
}

type LogFilter struct {
	StatusCode *int
	KeyID      *uint
	Limit      int
	Offset     int
}

func (s *LogService) List(ctx context.Context, f LogFilter) ([]models.RequestLog, int64, error) {
	q := s.db.WithContext(ctx).Model(&models.RequestLog{})
	if f.StatusCode != nil {
		q = q.Where("status_code = ?", *f.StatusCode)
	}
	if f.KeyID != nil {
		q = q.Where("key_id = ?", *f.KeyID)
	}
	var total int64
	q.Count(&total)
	if f.Limit <= 0 {
		f.Limit = 50
	}
	var logs []models.RequestLog
	err := q.Order("id desc").Offset(f.Offset).Limit(f.Limit).Find(&logs).Error
	return logs, total, err
}

func (s *LogService) Clear(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&models.RequestLog{}).Error
}

// Cleanup deletes logs older than the given number of days.
func (s *LogService) Cleanup(ctx context.Context, days int) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	result := s.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&models.RequestLog{})
	if result.Error != nil {
		log.Printf("Log cleanup failed: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("Log cleanup: deleted %d entries older than %d days", result.RowsAffected, days)
	}
}
