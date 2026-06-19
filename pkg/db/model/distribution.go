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

package model

import (
	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Distribution{})
}

// Distribution 部署支持的操作系统发行版
type Distribution struct {
	pixiu.Model

	Family      string `gorm:"type:varchar(32);not null;uniqueIndex:uk_family_version" json:"family"`
	Version     string `gorm:"type:varchar(64);not null;uniqueIndex:uk_family_version" json:"version"`
	EngineImage string `gorm:"type:varchar(255);not null" json:"engine_image"`
}

func (distribution *Distribution) TableName() string {
	return "distributions"
}
