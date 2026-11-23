/*
Copyright 2026 The Pixiu Authors.

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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	utilerrors "github.com/caoyingjunz/pixiu/pkg/util/errors"
)

// ResourceLabelInterface 平台级标签关联表读写接口，与资源类型无关。
type ResourceLabelInterface interface {
	// Set 整体覆盖一个资源的标签，为空时删除该行。
	Set(ctx context.Context, resourceType string, resourceId int64, labels model.Labels) error
	// Delete 删除资源时级联清理，避免孤儿行。
	Delete(ctx context.Context, resourceType string, resourceId int64) error
	// Get 读取单个资源的标签，不存在时返回空集合。
	Get(ctx context.Context, resourceType string, resourceId int64) (model.Labels, error)
	// BatchGet 一次取回多个资源的标签，避免列表页 N+1。
	BatchGet(ctx context.Context, resourceType string, ids []int64) (map[int64]model.Labels, error)
}

type resourceLabel struct {
	db *gorm.DB
}

func newResourceLabel(db *gorm.DB) ResourceLabelInterface { return &resourceLabel{db} }

func (r *resourceLabel) Set(ctx context.Context, resourceType string, resourceId int64, labels model.Labels) error {
	if len(labels) == 0 {
		return r.Delete(ctx, resourceType, resourceId)
	}
	now := time.Now()
	object := &model.ResourceLabel{
		ResourceType: resourceType,
		ResourceId:   resourceId,
		Labels:       labels,
	}
	object.GmtCreate = now
	object.GmtModified = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "resource_type"}, {Name: "resource_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"labels", "gmt_modified"}),
	}).Create(object).Error
}

func (r *resourceLabel) Delete(ctx context.Context, resourceType string, resourceId int64) error {
	return r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ?", resourceType, resourceId).
		Delete(&model.ResourceLabel{}).Error
}

func (r *resourceLabel) Get(ctx context.Context, resourceType string, resourceId int64) (model.Labels, error) {
	var object model.ResourceLabel
	err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id = ?", resourceType, resourceId).
		First(&object).Error
	if utilerrors.IsRecordNotFound(err) {
		return model.Labels{}, nil
	}
	if err != nil {
		return nil, err
	}
	return object.Labels, nil
}

func (r *resourceLabel) BatchGet(ctx context.Context, resourceType string, ids []int64) (map[int64]model.Labels, error) {
	result := make(map[int64]model.Labels, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var objects []model.ResourceLabel
	if err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_id IN ?", resourceType, ids).
		Find(&objects).Error; err != nil {
		return nil, err
	}
	for _, object := range objects {
		result[object.ResourceId] = object.Labels
	}
	return result, nil
}
