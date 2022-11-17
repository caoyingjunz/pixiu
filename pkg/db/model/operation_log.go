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

package model

import (
	"time"

	"gorm.io/gorm"
)

type OperationLog struct {
	Id         int64         `gorm:"column:id;primary_key;AUTO_INCREMENT;not null" json:"id"`                // 主键
	UserID     int64         `json:"user_id" form:"user_id" gorm:"column:user_id"`                           // 用户id
	GmtCreate  time.Time     `json:"gmt_create"`                                                             // 操作时间
	Ip         string        `json:"ip" form:"ip" gorm:"column:ip"`                                          // 客户端ip
	Location   string        `json:"location" form:"location" gorm:"column:location"`                        // 操作地址
	Agent      string        `json:"agent" form:"agent" gorm:"column:agent"`                                 // 浏览器类型
	Path       string        `json:"path" form:"path" gorm:"column:path"`                                    // 请求路径
	Method     string        `json:"method" form:"method" gorm:"column:method"`                              // 请求方法
	Param      string        `json:"param" form:"param" gorm:"type:longtext;column:param"`                   // 入参
	Status     int           `json:"status" form:"status" gorm:"column:status"`                              // 请求状态
	Latency    time.Duration `json:"latency" form:"latency" gorm:"column:latency"`                           // 延迟
	PespResult string        `json:"resp_result" form:"resp_result" gorm:"type:longtext;column:resp_result"` // 返回值
	ErrMsg     string        `json:"err_msg" form:"err_msg" gorm:"column:err_msg"`                           // 错误信息
	GmtDelete  time.Time     `json:"gmt_delete"  form:"gmt_delete" gorm:"column:gmt_delete"`                 // 删除时间
	DelFlag    int8          `json:"del_flag" form:"del_flag" gorm:"column:del_flag"`                        // 删除标记(0正常 1删除)
	User       User          `json:"user"`
}

// TableName 表名
func (a *OperationLog) TableName() string {
	return "audit_operation_log"
}

// BeforeCreate 添加前
func (ol *OperationLog) BeforeCreate(*gorm.DB) error {
	ol.GmtCreate = time.Now()
	return nil
}

// PageOperationLog 分页操作日志
type PageOperationLog struct {
	OperationLogs []OperationLog `json:"OperationLogs"`
	Total         int64          `json:"total"`
}

type IdsReq struct {
	Ids []int64 `json:"ids" form:"ids"`
}
