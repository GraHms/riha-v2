package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"riha/internal/models"
)

func Open(dsn string) (*gorm.DB, error) {
	lvl := logger.Warn
	if os.Getenv("RIHA_SQL_DEBUG") == "true" {
		lvl = logger.Info
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{SlowThreshold: 500 * time.Millisecond, LogLevel: lvl, IgnoreRecordNotFoundError: true}),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	return gdb, nil
}

func Migrate(gdb *gorm.DB) error {
	if err := gdb.AutoMigrate(models.All()...); err != nil {
		return fmt.Errorf("automigrate: %w", err)
	}
	return gdb.Exec(`CREATE SEQUENCE IF NOT EXISTS payout_number_seq START 1`).Error
}
