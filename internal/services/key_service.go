// internal/services/key_service.go
package services

import (
	"context"
	"log"
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

func (s *KeyService) GetByID(ctx context.Context, id uint) (*models.APIKey, error) {
	var k models.APIKey
	err := s.db.WithContext(ctx).First(&k, id).Error
	return &k, err
}

type KeyCandidate struct {
	ID          uint
	Key         string
	Alias       string
	UsedCount   int64
	MaxRequests int64
	LastUsedAt  *time.Time
}

func (s *KeyService) Candidates(ctx context.Context, strategy string) ([]KeyCandidate, error) {
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
		out = append(out, KeyCandidate{ID: k.ID, Key: k.Key, Alias: k.Alias, UsedCount: k.UsedCount, MaxRequests: k.MaxRequests, LastUsedAt: k.LastUsedAt})
	}
	switch strategy {
	case "round_robin":
		// Sort by last_used_at ascending (NULL first) — use the key that was used longest ago
		sort.Slice(out, func(i, j int) bool {
			li := out[i].LastUsedAt
			lj := out[j].LastUsedAt
			if li == nil && lj == nil {
				return out[i].ID < out[j].ID
			}
			if li == nil {
				return true
			}
			if lj == nil {
				return false
			}
			return li.Before(*lj)
		})
	case "random":
		rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	default: // "least_used"
		sort.Slice(out, func(i, j int) bool {
			if out[i].UsedCount != out[j].UsedCount {
				return out[i].UsedCount < out[j].UsedCount
			}
			return rand.Intn(2) == 0
		})
	}
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

func (s *KeyService) ResetMonthlyUsage(ctx context.Context) {
	if err := s.db.WithContext(ctx).Model(&models.APIKey{}).Where("used_count > 0").Update("used_count", 0).Error; err != nil {
		log.Printf("Monthly used_count reset failed: %v", err)
	} else {
		log.Println("Monthly used_count reset completed")
	}
}

func (s *KeyService) GetStrategy(ctx context.Context) string {
	var setting models.Setting
	if err := s.db.WithContext(ctx).Where("key = ?", "strategy").First(&setting).Error; err != nil {
		return "least_used"
	}
	if setting.Value == "" {
		return "least_used"
	}
	return setting.Value
}

func (s *KeyService) SetStrategy(ctx context.Context, strategy string) error {
	if strategy != "least_used" && strategy != "round_robin" && strategy != "random" {
		strategy = "least_used"
	}
	return s.db.WithContext(ctx).Save(&models.Setting{Key: "strategy", Value: strategy}).Error
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
