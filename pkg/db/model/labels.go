/*
Copyright 2026 The Pixiu Authors.

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

package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Labels 是平台级标签类型，以 JSON 对象存储。
// 任何需要标签的资源/关联表都可以直接使用，由 GORM 自动完成编解码。
type Labels map[string]string

// Value 实现 driver.Valuer，写入数据库时序列化为 JSON。
func (l Labels) Value() (driver.Value, error) {
	if l == nil {
		return nil, nil
	}
	b, err := json.Marshal(l)
	return string(b), err
}

// Scan 实现 sql.Scanner，读取数据库时反序列化；兼容 NULL、空串、string 与 []byte。
func (l *Labels) Scan(src any) error {
	if src == nil {
		*l = nil
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("unsupported labels column type %T", src)
	}
	if len(data) == 0 {
		*l = nil
		return nil
	}
	return json.Unmarshal(data, l)
}
