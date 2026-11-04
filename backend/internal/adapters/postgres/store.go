package postgres

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"riha/internal/core/domain"
)

type Store struct{ db *gorm.DB }

type gormDB = gorm.DB

type txKey struct{}

func Open(dsn string) (*Store, error) {
	lvl := logger.Warn
	if os.Getenv("RIHA_SQL_DEBUG") == "true" {
		lvl = logger.Info
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{SlowThreshold: 500 * time.Millisecond, LogLevel: lvl, IgnoreRecordNotFoundError: true, ParameterizedQueries: true}),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	return &Store{db: db}, nil
}

func (s *Store) Migrate() error {
	if err := s.db.AutoMigrate(allRecords()...); err != nil {
		return err
	}
	return s.db.Exec(`CREATE SEQUENCE IF NOT EXISTS payout_number_seq START 1`).Error
}

func (s *Store) DropAll() error {
	for _, r := range allRecords() {
		if err := s.db.Migrator().DropTable(r); err != nil {
			return err
		}
	}
	return s.db.Exec(`DROP SEQUENCE IF EXISTS payout_number_seq`).Error
}

func (s *Store) conn(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return s.db.WithContext(ctx)
}

func (s *Store) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

var forUpdate = clause.Locking{Strength: "UPDATE"}

func wrap(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func first[R any, D any](q *gorm.DB) (*D, error) {
	var r R
	if err := q.First(&r).Error; err != nil {
		return nil, wrap(err)
	}
	return toDomain[D](&r), nil
}

func find[R any, D any](q *gorm.DB) ([]D, error) {
	var rs []R
	if err := q.Find(&rs).Error; err != nil {
		return nil, err
	}
	return many[D](rs), nil
}

func create[R any, D any](db *gorm.DB, d *D) error { return db.Create(toRecord[R](d)).Error }

func save[R any, D any](db *gorm.DB, d *D) error {
	return db.Omit("created_at").Save(toRecord[R](d)).Error
}

func exists(q *gorm.DB) (bool, error) {
	var n int64
	err := q.Count(&n).Error
	return n > 0, err
}
