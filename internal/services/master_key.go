// internal/services/master_key.go
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type MasterKeyService struct {
	db        *gorm.DB
	key       string
	customKey string
}

func NewMasterKeyService(db *gorm.DB, customKey string) *MasterKeyService {
	return &MasterKeyService{db: db, customKey: customKey}
}

func (s *MasterKeyService) LoadOrCreate(ctx context.Context) error {
	if s.customKey != "" {
		s.key = s.customKey
		s.db.WithContext(ctx).Where("id = 1").
			Assign(models.MasterKey{Key: s.customKey}).
			FirstOrCreate(&models.MasterKey{ID: 1, Key: s.customKey})
		return nil
	}
	var mk models.MasterKey
	if err := s.db.WithContext(ctx).First(&mk).Error; err == nil {
		s.key = mk.Key
		return nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate master key: %w", err)
	}
	s.key = hex.EncodeToString(b)
	return s.db.WithContext(ctx).Create(&models.MasterKey{ID: 1, Key: s.key}).Error
}

func (s *MasterKeyService) Get() string    { return s.key }
func (s *MasterKeyService) Validate(k string) bool { return k == s.key }

func (s *MasterKeyService) Reset(ctx context.Context) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s.key = hex.EncodeToString(b)
	return s.key, s.db.WithContext(ctx).Model(&models.MasterKey{}).
		Where("id = 1").Update("key", s.key).Error
}
