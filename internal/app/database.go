package app

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	appconfig "github.com/zatrano/zbe/config"
	zaplogger "github.com/zatrano/zbe/pkg/logger"
)

// NewDatabase opens a GORM connection to PostgreSQL with connection pooling.
func NewDatabase(cfg *appconfig.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	}

	if cfg.IsDevelopment() {
		gormCfg.Logger = logger.Default.LogMode(logger.Info)
	} else {
		gormCfg.Logger = logger.Default.LogMode(logger.Warn)
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	// Verify connectivity
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("database ping: %w", err)
	}

	zaplogger.Info("database connected successfully")
	return db, nil
}

// CloseDatabase gracefully closes the underlying sql.DB.
func CloseDatabase(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		zaplogger.Errorf("close db get sql.DB: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		zaplogger.Errorf("close database: %v", err)
	}
}
