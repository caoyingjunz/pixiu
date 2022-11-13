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

package audit

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

// OperationLogInterface 登录日志数据访问层
type OperationLogInterface interface {
	Create(c context.Context, obj *model.OperationLog) (operationLog *model.OperationLog, err error)
	Delete(c context.Context, ids []int64) error
	List(c context.Context, page, limit int) (res *model.PageOperationLog, err error)
}

type operationLog struct {
	db *gorm.DB
}

func NewOperationLog(db *gorm.DB) OperationLogInterface {
	return &operationLog{db}
}

func (ol operationLog) Create(c context.Context, obj *model.OperationLog) (operationLog *model.OperationLog, err error) {
	if err := ol.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (ol operationLog) Delete(c context.Context, ids []int64) error {
	mp := map[string]interface{}{"gmt_delete": time.Now(), "del_flag": types.Deleted}
	if err := ol.db.Debug().Table("audit_operation_log").Where("id IN ?", ids).Updates(mp); err != nil {
		return err.Error
	}
	return nil
}

func (ol operationLog) List(c context.Context, page, limit int) (res *model.PageOperationLog, err error) {
	var (
		operationLogList []model.OperationLog
		total            int64
	)

	if page == 0 && limit == 0 {
		// 增量查询
		if tx := ol.db.Where("del_flag", types.Normal).Find(&operationLogList); tx.Error != nil {
			return nil, tx.Error
		}
		if err := ol.db.Model(&model.OperationLog{}).Count(&total).Error; err != nil {
			return nil, err
		}
		res := &model.PageOperationLog{
			OperationLogs: operationLogList,
			Total:         total,
		}
		return res, err
	}

	// 分页数据
	if err := ol.db.Limit(limit).Offset((page-1)*limit).
		Where("del_flag", types.Normal).
		Find(&operationLogList).Error; err != nil {
		return nil, err
	}
	if err := ol.db.Model(&model.OperationLog{}).Count(&total).Error; err != nil {
		return nil, err
	}
	res = &model.PageOperationLog{
		OperationLogs: operationLogList,
		Total:         total,
	}
	return res, err
}
