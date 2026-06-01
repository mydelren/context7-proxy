// internal/services/key_service.go
package services

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type KeyService struct {
	db          *gorm.DB
	cooldownSec int
}

func NewKeyService(db *gorm.DB, cooldownSec int) *KeyService {
	return &KeyService{db: db, cooldownSec: cooldownSec}
}

func (s *KeyService) List(ctx context.Context) ([]models.APIKey, error) {
	var keys []models.APIKey
	return keys, s.db.WithContext(ctx).Order("id desc").Find(&keys).Error
}

func (s *KeyService) Create(ctx context.Context, key, alias string) (*models.APIKey, error) {
	k := models.APIKey{Key: key, Alias: alias, IsActive: true}
	return &k, s.db.WithContext(ctx).Create(&k).Error
}

func (s *KeyService) Update(ctx context.Context, id uint, alias *string, isActive *bool, maxRequests *int64) (*models.APIKey, error) {
	var k models.APIKey
	if err := s.db.WithContext(ctx).First(&k, id).Error; err != nil {
		return nil, err
	}
	if alias != nil {
		k.Alias = *alias
	}
	if isActive != nil {
		if k.IsInvalid && *isActive {
			k.IsActive = false
		} else {
			k.IsActive = *isActive
		}
	}
	if maxRequests != nil {
		k.MaxRequests = *maxRequests
	}
	return &k, s.db.WithContext(ctx).Save(&k).Error
}

func (s *KeyService) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.APIKey{}, id).Error
}

func (s *KeyService) GetRaw(ctx context.Context, id uint) (string, error) {
	var k models.APIKey
	err := s.db.WithContext(ctx).Select("key").First(&k, id).Error
	return k.Key, err
}

type KeyCandidate struct {
	ID          uint
	Key         string
	Alias       string
	UsedCount   int64
	MaxRequests int64
}

func (s *KeyService) Candidates(ctx context.Context) ([]KeyCandidate, error) {
	var keys []models.APIKey
	now := time.Now()
	if err := s.db.WithContext(ctx).
		Where("is_invalid = ? AND is_active = ?", false, true).
		Find(&keys).Error; err != nil {
		return nil, err
	}
	var out []KeyCandidate
	for _, k := range keys {
		if k.CooldownAt != nil && k.CooldownAt.After(now) {
			continue
		}
		if k.MaxRequests > 0 && k.UsedCount >= k.MaxRequests {
			continue
		}
		out = append(out, KeyCandidate{ID: k.ID, Key: k.Key, Alias: k.Alias, UsedCount: k.UsedCount, MaxRequests: k.MaxRequests})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UsedCount != out[j].UsedCount {
			return out[i].UsedCount < out[j].UsedCount
		}
		return rand.Intn(2) == 0
	})
	return out, nil
}

func (s *KeyService) MarkUsed(ctx context.Context, id uint) {
	now := time.Now()
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).
		Updates(map[string]interface{}{"used_count": gorm.Expr("used_count + 1"), "last_used_at": now})
}

func (s *KeyService) MarkCooldown(ctx context.Context, id uint) {
	cd := time.Now().Add(time.Duration(s.cooldownSec) * time.Second)
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).Update("cooldown_at", cd)
}

func (s *KeyService) MarkInvalid(ctx context.Context, id uint) {
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_invalid": true, "is_active": false})
}

func (s *KeyService) DeleteInvalid(ctx context.Context) (int64, error) {
	r := s.db.WithContext(ctx).Where("is_invalid = ?", true).Delete(&models.APIKey{})
	return r.RowsAffected, r.Error
}

func (s *KeyService) Stats(ctx context.Context) (total, active, cooling, invalid int, err error) {
	var keys []models.APIKey
	if err = s.db.WithContext(ctx).Find(&keys).Error; err != nil {
		return
	}
	now := time.Now()
	for _, k := range keys {
		total++
		if k.IsInvalid {
			invalid++
		} else if k.IsActive {
			if k.CooldownAt != nil && k.CooldownAt.After(now) {
				cooling++
			}
			active++
		}
	}
	return
}
