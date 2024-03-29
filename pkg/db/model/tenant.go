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

import "github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"

func init() {
	register(&Tenant{})
}

type Tenant struct {
	pixiu.Model

	Name        string `gorm:"index:idx_name,unique" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Extension   string `gorm:"type:text" json:"extension,omitempty"`
}

func (tenant *Tenant) TableName() string {
	return "tenants"
}
