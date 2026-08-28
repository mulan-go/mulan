package session

import (
	"crypto/rand"
	"encoding"
	"encoding/hex"
	"encoding/json"
	"io"
	"slices"
	"sync"
)

type (
	DataStringSlice []string
	DataMap         map[string]any
	Data            interface {
		Token() string
		ID() uint64
		Account() string
		State() uint16
		Roles() []string
		Items() DataMap
		Get(string) any
		New() string
		Set(string, any) Data
		SetToken(string) Data
		SetID(uint64) Data
		SetAccount(string) Data
		SetState(uint16) Data
		SetValues(string, any) Data
		SetRoles([]string) Data
		Delete(string) Data
		Clear() Data
	}
)

var _ encoding.TextUnmarshaler = (*DataStringSlice)(nil)
var _ encoding.TextUnmarshaler = (*DataMap)(nil)

func (d DataStringSlice) MarshalBinary() ([]byte, error) {
	return json.Marshal(d)
}

func (d *DataStringSlice) UnmarshalText(buf []byte) error {
	return d.UnmarshalBinary(buf)
}

func (d *DataStringSlice) UnmarshalJSON(buf []byte) error {
	return d.UnmarshalText(buf)
}

func (d *DataStringSlice) UnmarshalBinary(buf []byte) error {
	v := []string{}
	err := json.Unmarshal(buf, &v)
	if err != nil {
		return err
	}
	*d = DataStringSlice(v)
	return nil
}

func (d DataMap) MarshalBinary() ([]byte, error) { return json.Marshal(d) }

func (d *DataMap) UnmarshalBinary(buf []byte) error { return json.Unmarshal(buf, d) }

func (d *DataMap) UnmarshalJSON(buf []byte) error { return d.UnmarshalText(buf) }

func (d *DataMap) UnmarshalText(buf []byte) error {
	_d := make(map[string]any)
	if err := json.Unmarshal(buf, &_d); err != nil {
		return err
	}
	*d = _d
	return nil
}

type DefaultData struct {
	mu       sync.RWMutex    `json:"-" redis:"-"`
	Items_   DataMap         `json:"items" redis:"items"`
	Token_   string          `json:"token" redis:"token"`
	Account_ string          `json:"account" redis:"account"`
	Roles_   DataStringSlice `json:"roles" redis:"roles"`
	ID_      uint64          `json:"id" redis:"id"`
	State_   uint16          `json:"state" redis:"state"`
}

var _ Data = (*DefaultData)(nil)

func New() string {
	k := make([]byte, 20)
	io.ReadFull(rand.Reader, k)
	return hex.EncodeToString(k)
}

func (d *DefaultData) New() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Token_ = New()
	return d.Token_
}

func (d *DefaultData) ID() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.ID_
}

func (d *DefaultData) Token() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Token_
}

func (d *DefaultData) Account() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Account_
}

func (d *DefaultData) State() uint16 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.State_
}

func (d *DefaultData) Roles() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.Roles_ == nil {
		return nil
	}
	return slices.Clone([]string(d.Roles_))
}

func (d *DefaultData) Items() DataMap {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.Items_ == nil {
		return nil
	}
	m := make(DataMap, len(d.Items_))
	for k, v := range d.Items_ {
		m[k] = v
	}
	return m
}

func (d *DefaultData) SetToken(v string) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Token_ = v
	return d
}

func (d *DefaultData) SetID(v uint64) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ID_ = v
	return d
}

func (d *DefaultData) SetAccount(v string) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Account_ = v
	return d
}

func (d *DefaultData) SetState(v uint16) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.State_ = v
	return d
}

func (d *DefaultData) SetRoles(v []string) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Roles_ = DataStringSlice(v)
	return d
}

func (d *DefaultData) SetValues(k string, v any) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Items_ == nil {
		d.Items_ = make(DataMap)
	}
	d.Items_[k] = v
	return d
}

func (d *DefaultData) Set(key string, val any) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch key {
	case "id":
		if v, ok := val.(uint64); ok {
			d.ID_ = v
		}
	case "account":
		if v, ok := val.(string); ok {
			d.Account_ = v
		}
	case "roles":
		if v, ok := val.([]string); ok {
			d.Roles_ = DataStringSlice(v)
		}
	default:
		if d.Items_ == nil {
			d.Items_ = make(DataMap)
		}
		d.Items_[key] = val
	}
	return d
}

func (d *DefaultData) Get(key string) any {
	d.mu.RLock()
	defer d.mu.RUnlock()
	switch key {
	case "id":
		return d.ID_
	case "account":
		return d.Account_
	case "roles":
		return slices.Clone([]string(d.Roles_))
	default:
		if d.Items_ != nil {
			if v, ok := d.Items_[key]; ok {
				return v
			}
		}
	}
	return nil
}

func (d *DefaultData) Delete(key string) Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch key {
	case "id":
		d.ID_ = 0
	case "account":
		d.Account_ = ""
	case "roles":
		d.Roles_ = nil
	default:
		if d.Items_ != nil {
			delete(d.Items_, key)
		}
	}
	return d
}

func (d *DefaultData) Clear() Data {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Items_ = nil
	d.Token_ = ""
	d.Account_ = ""
	d.Roles_ = nil
	d.ID_ = 0
	d.State_ = 0
	return d
}
