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

// TimeOption 通用时间规格
type TimeOption struct {
	GmtCreate   interface{} `json:"gmt_create,omitempty"`
	GmtModified interface{} `json:"gmt_modified,omitempty"`
}

// PageOptions 分页选项
type PageOptions struct {
	Limit int `form:"limit"`
	Page  int `form:"page"`
}

type IdMeta struct {
	Id int64 `uri:"id" binding:"required"`
}

type CloudMeta struct {
	CloudName string `uri:"cloud_name" binding:"required"`
}

type CloudIdMeta struct {
	CloudMeta `json:",inline"`
	IdMeta    `json:",inline"`
}
