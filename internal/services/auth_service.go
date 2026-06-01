// internal/services/auth_service.go
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mydelren/context7-proxy/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const tokenTTL = 24 * time.Hour

type AuthService struct {
	db          *gorm.DB
	mu          sync.RWMutex
	adminTokens map[string]time.Time
	legacyKey   string
}

func NewAuthService(db *gorm.DB, legacyKey string) *AuthService {
	a := &AuthService{
		db:          db,
		adminTokens: make(map[string]time.Time),
		legacyKey:   legacyKey,
	}
	go a.cleanupLoop()
	return a
}

func (a *AuthService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		a.CleanupTokens()
	}
}

func (a *AuthService) CleanupTokens() {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for tok, created := range a.adminTokens {
		if now.Sub(created) > tokenTTL {
			delete(a.adminTokens, tok)
		}
	}
}

func (a *AuthService) Login(username, password string) (string, error) {
	var admin models.Admin
	if err := a.db.Where("username = ?", username).First(&admin).Error; err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid credentials")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	a.mu.Lock()
	a.adminTokens[token] = time.Now()
	a.mu.Unlock()
	return token, nil
}

func (a *AuthService) Validate(token string) (role string, memberID uint, memberName string, apiKeyID *uint) {
	// 1) admin in-memory token
	a.mu.RLock()
	created, ok := a.adminTokens[token]
	a.mu.RUnlock()
	if ok && time.Since(created) <= tokenTTL {
		return "admin", 0, "", nil
	}

	// 2) member DB token
	var m models.Member
	if err := a.db.Where("token = ? AND is_active = ?", token, true).First(&m).Error; err == nil {
		return "member", m.ID, m.Name, m.APIKeyID
	}

	// 3) legacy MasterKey
	if a.legacyKey != "" && token == a.legacyKey {
		return "admin", 0, "", nil
	}

	return "", 0, "", nil
}

func (a *AuthService) CreateMember(ctx context.Context, name string) (*models.Member, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	m := &models.Member{
		Name:     name,
		Token:    hex.EncodeToString(b),
		IsActive: true,
	}
	return m, a.db.WithContext(ctx).Create(m).Error
}

func (a *AuthService) ListMembers(ctx context.Context) ([]models.Member, error) {
	var members []models.Member
	if err := a.db.WithContext(ctx).Order("id desc").Find(&members).Error; err != nil {
		return nil, err
	}
	for i := range members {
		tok := members[i].Token
		if len(tok) > 8 {
			members[i].TokenSuffix = tok[len(tok)-8:]
		} else {
			members[i].TokenSuffix = tok
		}
	}
	return members, nil
}

func (a *AuthService) DeleteMember(ctx context.Context, id uint) error {
	r := a.db.WithContext(ctx).Delete(&models.Member{}, id)
	if r.RowsAffected == 0 {
		return fmt.Errorf("member not found")
	}
	return r.Error
}

func (a *AuthService) UpdateMemberKey(ctx context.Context, id uint, apiKeyID *uint) error {
	var m models.Member
	if err := a.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return fmt.Errorf("member not found")
	}
	return a.db.WithContext(ctx).Model(&m).Updates(map[string]interface{}{"api_key_id": apiKeyID}).Error
}

func (a *AuthService) LegacyKey() string { return a.legacyKey }

func (a *AuthService) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	var admin models.Admin
	if err := a.db.WithContext(ctx).Where("username = ?", username).First(&admin).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("old password is incorrect")
	}
	if len(newPassword) < 6 {
		return fmt.Errorf("new password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.db.WithContext(ctx).Model(&admin).Update("password_hash", string(hash)).Error
}

func (a *AuthService) InitAdmin(ctx context.Context) error {
	var count int64
	a.db.WithContext(ctx).Model(&models.Admin{}).Count(&count)
	if count > 0 {
		return nil
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	password := hex.EncodeToString(b)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := &models.Admin{Username: "admin", PasswordHash: string(hash)}
	if err := a.db.WithContext(ctx).Create(admin).Error; err != nil {
		return err
	}
	log.Printf("Admin account created — username: admin, password: %s", password)
	return nil
}
