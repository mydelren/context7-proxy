// internal/db/db.go
package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := gormDB.AutoMigrate(&models.APIKey{}, &models.RequestLog{}, &models.MasterKey{}, &models.Admin{}, &models.Member{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return gormDB, nil
}
