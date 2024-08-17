/*
Copyright 2021 The Pixiu Authors.

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
	"strconv"
	"time"
)

type Model struct {
	Id              int64     `gorm:"column:id;primaryKey;autoIncrement;not null" json:"id"`
	GmtCreate       time.Time `gorm:"column:gmt_create;type:datetime;default:current_timestamp;not null" json:"gmt_create"`
	GmtModified     time.Time `gorm:"column:gmt_modified;type:datetime;default:current_timestamp;not null" json:"gmt_modified"`
	ResourceVersion int64     `gorm:"column:resource_version;default:0;not null" json:"resource_version"`
}

func (m Model) GetSID() string {
	return strconv.FormatInt(m.Id, 10)
}
