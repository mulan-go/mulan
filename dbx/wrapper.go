package dbx

import "gorm.io/gorm"

type (
	MigratorFunc     func(db *gorm.DB) gorm.Migrator
	DialectorWrapper struct {
		gorm.Dialector
		migrator_ MigratorFunc
	}
)

func (d *DialectorWrapper) Apply(dialector gorm.Dialector) {
	d.Dialector = dialector
}
func (d *DialectorWrapper) SetMigrator(migrator MigratorFunc) {
	d.migrator_ = migrator
}
func (d *DialectorWrapper) Migrator(db *gorm.DB) gorm.Migrator {
	if d.migrator_ != nil {
		return d.migrator_(db)
	}
	return d.Dialector.Migrator(db)
}

func NewDialectorWrapper() *DialectorWrapper {
	return &DialectorWrapper{}
}
