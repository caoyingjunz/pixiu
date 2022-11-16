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

package meta

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	offset = 1  // 默认取第一页
	limit  = 10 // 默认取10条记录
)

// ListSelector is a query selector for list APIs
type ListSelector struct {
	// 搜索类型，目前支持 name
	Field string

	Page  int // 页数
	Limit int // 每页数量
}

func (s *ListSelector) SetField(field string) {
	s.Field = field
}

func ParseListSelector(c *gin.Context) *ListSelector {
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		page = offset
	}
	pageSize, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		pageSize = limit
	}

	ls := &ListSelector{
		Page:  page,
		Limit: pageSize,
	}
	ls.SetField("name")
	return ls
}
