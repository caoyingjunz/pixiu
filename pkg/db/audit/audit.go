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

// AuditInterface 审计数据访问层
type AuditInterface interface {
	Create(c context.Context, obj *model.Audit) (audit *model.Audit, err error)
	Delete(c context.Context, ids []int64) error
	List(c context.Context, page, limit int) (res *model.PageAudit, err error)
}

type audit struct {
	db *gorm.DB
}

func NewAudit(db *gorm.DB) AuditInterface {
	return &audit{db}
}

func (a audit) Create(c context.Context, obj *model.Audit) (audit *model.Audit, err error) {
	if err := a.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (a audit) Delete(c context.Context, ids []int64) error {
	mp := map[string]interface{}{"gmt_delete": time.Now(), "del_flag": types.Deleted}
	if err := a.db.Debug().Table("audit").Where("id IN ?", ids).Updates(mp); err != nil {
		return err.Error
	}
	return nil
}

func (a audit) List(c context.Context, page, limit int) (res *model.PageAudit, err error) {
	var (
		auditList []model.Audit
		total     int64
	)

	if page == 0 && limit == 0 {
		// 增量查询
		if tx := a.db.Where("del_flag", types.Normal).Find(&auditList); tx.Error != nil {
			return nil, tx.Error
		}
		if err := a.db.Model(&model.Audit{}).Count(&total).Error; err != nil {
			return nil, err
		}
		res := &model.PageAudit{
			Audits: auditList,
			Total:  total,
		}
		return res, err
	}

	// 分页数据
	if err := a.db.Limit(limit).Offset((page-1)*limit).
		Where("del_flag", types.Normal).
		Find(&auditList).Error; err != nil {
		return nil, err
	}
	if err := a.db.Model(&model.Audit{}).Count(&total).Error; err != nil {
		return nil, err
	}
	res = &model.PageAudit{
		Audits: auditList,
		Total:  total,
	}
	return res, err
}
