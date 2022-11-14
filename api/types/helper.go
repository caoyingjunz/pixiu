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

package types

import "time"

type IdUriMeta struct {
	Id int64 `uri:"id" binding:"required"`
}

type CloudUriMeta struct {
	IdUriMeta `json:",inline"`

	CloudName string `uri:"cloud_name" binding:"required"`
}

// TODO: 合并 IdUriMeta
type IdMeta struct {
	Id              int64 `json:"id"`
	ResourceVersion int64 `json:"resource_version"`
}

// TimeOption 通用时间规格
type TimeOption struct {
	GmtCreate   interface{} `json:"gmt_create,omitempty"`
	GmtModified interface{} `json:"gmt_modified,omitempty"`
}

const (
	timeLayout = "2006-01-02 15:04:05.999999999"
)

func FormatTime(GmtCreate time.Time, GmtModified time.Time) TimeOption {
	return TimeOption{
		GmtCreate:   GmtCreate.Format(timeLayout),
		GmtModified: GmtModified.Format(timeLayout),
	}
}
