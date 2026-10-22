package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type StringArray []string

// Value implements the driver.Valuer interface, used to convert StringArray to database value
func (c StringArray) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface, used to convert database value to StringArray
// Scan 同时接受 SQLite JSON TEXT 和 BLOB；未知类型不能被静默视作空数组。
func (c *StringArray) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}
	var raw []byte
	switch value := value.(type) {
	case []byte:
		raw = value
	case string:
		raw = []byte(value)
	default:
		return fmt.Errorf("invalid JSON array SQL type %T", value)
	}
	return json.Unmarshal(raw, c)
}
