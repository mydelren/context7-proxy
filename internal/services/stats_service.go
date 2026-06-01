// internal/services/stats_service.go
package services

import (
	"context"
	"time"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type StatsService struct{ db *gorm.DB }

func NewStatsService(db *gorm.DB) *StatsService { return &StatsService{db: db} }

type Stats struct {
	TotalRequests int64 `json:"total_requests"`
	TodayRequests int64 `json:"today_requests"`
	RateLimited   int64 `json:"rate_limited"`
	TotalKeys     int   `json:"total_keys"`
	ActiveKeys    int   `json:"active_keys"`
	CoolingKeys   int   `json:"cooling_keys"`
	InvalidKeys   int   `json:"invalid_keys"`
}

func (s *StatsService) Get(ctx context.Context, keyStats func(context.Context) (int, int, int, int, error)) (Stats, error) {
	var st Stats
	s.db.WithContext(ctx).Model(&models.RequestLog{}).Count(&st.TotalRequests)
	today := time.Now().Truncate(24 * time.Hour)
	s.db.WithContext(ctx).Model(&models.RequestLog{}).Where("created_at >= ?", today).Count(&st.TodayRequests)
	s.db.WithContext(ctx).Model(&models.RequestLog{}).Where("status_code = ?", 429).Count(&st.RateLimited)
	t, a, c, i, err := keyStats(ctx)
	st.TotalKeys, st.ActiveKeys, st.CoolingKeys, st.InvalidKeys = t, a, c, i
	return st, err
}

type TimeSeriesPoint struct {
	Hour  string `json:"hour"`
	Count int64  `json:"count"`
}

func (s *StatsService) TimeSeries(ctx context.Context, hours int) ([]TimeSeriesPoint, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.db.WithContext(ctx).Raw(`
		SELECT strftime('%Y-%m-%d %H:00', created_at, 'localtime') as hour, count(*) as count
		FROM request_logs WHERE created_at >= ? GROUP BY hour ORDER BY hour
	`, since).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pts []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		rows.Scan(&p.Hour, &p.Count)
		pts = append(pts, p)
	}
	return pts, nil
}
