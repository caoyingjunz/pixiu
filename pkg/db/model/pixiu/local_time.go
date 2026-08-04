/*
Copyright 2024 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pixiu

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LocalTime 兼容人大金仓等驱动将 timestamp 以 string 返回的场景。
// 标准 database/sql 无法把 string Scan 进 time.Time，会报：
// unsupported Scan, storing driver.Value type string into type *time.Time
type LocalTime time.Time

var (
	_ sql.Scanner   = (*LocalTime)(nil)
	_ driver.Valuer = LocalTime{}
)

func LocalNow() LocalTime { return LocalTime(time.Now()) }

func AsLocalTime(t time.Time) LocalTime { return LocalTime(t) }

func AsLocalTimePtr(t *time.Time) *LocalTime {
	if t == nil {
		return nil
	}
	v := LocalTime(*t)
	return &v
}

func (t LocalTime) StdTime() time.Time { return time.Time(t) }

func (t *LocalTime) StdTimePtr() *time.Time {
	if t == nil {
		return nil
	}
	v := time.Time(*t)
	return &v
}

func (t LocalTime) IsZero() bool { return time.Time(t).IsZero() }

func (t LocalTime) Format(layout string) string { return time.Time(t).Format(layout) }

func (t LocalTime) Before(u time.Time) bool { return time.Time(t).Before(u) }

func (t LocalTime) After(u time.Time) bool { return time.Time(t).After(u) }

func (t LocalTime) Add(d time.Duration) time.Time { return time.Time(t).Add(d) }

func (t LocalTime) Sub(u time.Time) time.Duration { return time.Time(t).Sub(u) }

// GormDataType 告诉 gorm 底层列类型。
func (LocalTime) GormDataType() string { return "timestamp" }

func (t *LocalTime) Scan(value interface{}) error {
	if t == nil {
		return fmt.Errorf("LocalTime: Scan on nil receiver")
	}
	if value == nil {
		*t = LocalTime{}
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*t = LocalTime(v)
		return nil
	case LocalTime:
		*t = v
		return nil
	case string:
		parsed, err := parseFlexibleTime(v)
		if err != nil {
			return err
		}
		*t = LocalTime(parsed)
		return nil
	case []byte:
		parsed, err := parseFlexibleTime(string(v))
		if err != nil {
			return err
		}
		*t = LocalTime(parsed)
		return nil
	default:
		return fmt.Errorf("LocalTime: cannot scan type %T", value)
	}
}

func (t LocalTime) Value() (driver.Value, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return nil, nil
	}
	return tt, nil
}

func (t LocalTime) MarshalJSON() ([]byte, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte(`""`), nil
	}
	return json.Marshal(tt)
}

func (t *LocalTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		*t = LocalTime{}
		return nil
	}
	var tt time.Time
	if err := json.Unmarshal(data, &tt); err != nil {
		// 兼容纯字符串时间
		var s string
		if err2 := json.Unmarshal(data, &s); err2 != nil {
			return err
		}
		parsed, err3 := parseFlexibleTime(s)
		if err3 != nil {
			return err
		}
		*t = LocalTime(parsed)
		return nil
	}
	*t = LocalTime(tt)
	return nil
}

func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, fmt.Errorf("LocalTime: parse %q: %v", s, lastErr)
}
