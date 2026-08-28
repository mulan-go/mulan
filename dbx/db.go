package dbx

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gLogger "gorm.io/gorm/logger"
	zapgorm2 "moul.io/zapgorm2"
)

func NewLogger(isDebug bool) gLogger.Interface {
	logger := zapgorm2.New(zap.L())
	logger.IgnoreRecordNotFoundError = true
	logger.SetAsDefault()
	if isDebug {
		logger.LogMode(gLogger.Info)
	}
	return logger
}

func DefaultConfig() *gorm.Config {
	return &gorm.Config{
		QueryFields:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
		IgnoreRelationshipsWhenMigrating:         true,
		// PrepareStmt:                              true,
	}
}

func New(dialector gorm.Dialector, cfg *Config,
	opts ...gorm.Option) (*gorm.DB, error) {
	// Open
	db, err := gorm.Open(dialector, append(opts, DefaultConfig())...)
	if err != nil {
		zap.L().Error("Failed to open db",
			zap.String("name", dialector.Name()),
			zap.Error(err),
		)
		return nil, err
	}
	// sql.DB Config
	if cfg.Conn != nil {
		sqlDB, err := db.DB()
		if err != nil {
			zap.L().Warn("Failed to get sql.db", zap.Error(err))
		} else {
			sqlDB.SetMaxIdleConns(cfg.Conn.Idle)                                     // 最大空闲连接
			sqlDB.SetMaxOpenConns(cfg.Conn.Open)                                     // 最大连接数
			sqlDB.SetConnMaxLifetime(time.Duration(cfg.Conn.Lifetime) * time.Second) // 最大可复用时间
		}
	}
	if cfg.Debug {
		db = db.Debug()
	}
	return db, nil
}
