package dbx

import (
	"errors"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Auto(cfg *Config) (*gorm.DB, error) {
	cfg.Parse()
	gormCfg := &gorm.Config{
		Logger: NewLogger(cfg.Debug),
	}
	var dialector gorm.Dialector
	switch cfg.scheme {
	case "mysql":
		dialector = mysql.New(mysql.Config{
			DSN:                    cfg.String(),
			DefaultStringSize:      255,
			DontSupportRenameIndex: true,
		})
		return New(dialector, cfg, gormCfg)
	case "postgres":
		dialector = postgres.New(postgres.Config{
			DSN:                  cfg.String(),
			PreferSimpleProtocol: true,
		})
	case "sqlite", "sqlite3":
		dialector = sqlite.Open(cfg.String())
	default:
		return nil, errors.New("unsupported scheme: " + cfg.scheme)
	}
	return New(dialector, cfg, gormCfg)
}
