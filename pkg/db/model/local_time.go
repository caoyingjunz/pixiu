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

// LocalTime / LocalNow 便捷转发，供 db 层赋值使用。
type LocalTime = pixiu.LocalTime

func LocalNow() LocalTime                    { return pixiu.LocalNow() }
func AsLocalTime(t time.Time) LocalTime      { return pixiu.AsLocalTime(t) }
func AsLocalTimePtr(t *time.Time) *LocalTime { return pixiu.AsLocalTimePtr(t) }
