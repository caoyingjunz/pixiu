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
	"time"

	"github.com/caoyingjunz/pixiu/pkg/db/model/pixiu"
)

func init() {
	register(&Repository{})
}

type Repository struct {
	pixiu.Model
	Name     string `gorm:"column:name; index:idx_name,unique; not null" json:"name"`
	URL      string `gorm:"column:url;not null" json:"url"`
	Username string `gorm:"column:username" json:"username"`
	Password string `gorm:"column:password" json:"password"`
}

func (*Repository) TableName() string {
	return "repositories"
}

type ChartIndex struct {
	APIVersion string  `json:"apiVersion"`
	Entries    Entries `json:"entries"`
}

type Entries map[string][]ChartVersion

type ChartVersion struct {
	Annotations  map[string]string `json:"annotations"`
	APIVersion   string            `json:"apiVersion"`
	AppVersion   string            `json:"appVersion"`
	Created      time.Time         `json:"created"`
	Dependencies []Dependency      `json:"dependencies"`
	Description  string            `json:"description"`
	Digest       string            `json:"digest"`
	Icon         string            `json:"icon"`
	Maintainers  []Maintainer      `json:"maintainers"`
	Name         string            `json:"name"`
	Sources      []string          `json:"sources"`
	Type         string            `json:"type"`
	URLs         []string          `json:"urls"`
	Version      string            `json:"version"`
}

type Dependency struct {
	Condition  string `json:"condition"`
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Version    string `json:"version"`
	Alias      string `json:"alias,omitempty"`
}

type Maintainer struct {
	Name string `json:"name"`
}
