package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

// Permissions is a slice of Permission that serialises to/from JSONB.
type Permissions []Permission

func (p Permissions) Value() (driver.Value, error) {
	if p == nil {
		return "[]", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("permissions marshal: %w", err)
	}
	return string(b), nil
}

func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		*p = Permissions{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("permissions: unsupported scan type")
	}
	return json.Unmarshal(bytes, p)
}

// JSONB is a generic map that serialises to/from PostgreSQL JSONB.
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("jsonb marshal: %w", err)
	}
	return string(b), nil
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("jsonb: unsupported scan type")
	}
	return json.Unmarshal(bytes, j)
}
