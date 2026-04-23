package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON is a flexible map type for metadata fields.
// Stored as JSONB in PostgreSQL, serialized as a JSON object in responses.
type JSON map[string]any

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	b, err := json.Marshal(j)
	return string(b), err
}

func (j *JSON) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		s, ok := value.(string)
		if !ok {
			return errors.New("failed to scan JSON: unexpected type")
		}
		bytes = []byte(s)
	}
	return json.Unmarshal(bytes, j)
}
