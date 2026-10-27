package main

import (
	"fmt"
	"time"
)

// databaseTime accepts both MySQL's parsed dates and SQLite's stored UTC dates.
type databaseTime struct {
	Time  time.Time
	Valid bool
}

func (t *databaseTime) Scan(value any) error {
	*t = databaseTime{}
	if value == nil {
		return nil
	}
	if v, ok := value.(time.Time); ok {
		t.Time = v.UTC()
		t.Valid = true
		return nil
	}
	var text string
	switch v := value.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		return fmt.Errorf("unsupported database timestamp type %T", value)
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999", "2006-01-02"} {
		parsed, err := time.Parse(layout, text)
		if err == nil {
			t.Time = parsed.UTC()
			t.Valid = true
			return nil
		}
	}
	return fmt.Errorf("invalid database timestamp")
}
