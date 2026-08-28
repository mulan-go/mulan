package dbx

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type (
	StringSlice = datatypes.JSONSlice[string]

	Modeler interface {
		TableName() string
		Defaults() []Modeler
		GetID() uint64
	}
)

type Model struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	UUID      uuid.UUID `gorm:"type:uuid;unique;column:uuid"`
	CreatedAt int64     `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64     `gorm:"autoUpdateTime;column:updated_at"`
}

func (m *Model) Defaults() []Modeler { return []Modeler{} }
func (m *Model) GetID() uint64       { return m.ID }

func (m *Model) BeforeCreate(tx *gorm.DB) (err error) {
	if m.UUID == uuid.Nil {
		m.UUID, err = uuid.NewV7()
		if err != nil {
			m.UUID = uuid.New()
		}
	}
	return
}
