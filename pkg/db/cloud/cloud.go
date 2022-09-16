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

package cloud

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type CloudInterface interface {
	Create(ctx context.Context, obj *model.Cloud) (*model.Cloud, error)
	Update(ctx context.Context, cid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, cid int64) error
	Get(ctx context.Context, cid int64) (*model.Cloud, error)
	List(ctx context.Context) ([]model.Cloud, error)
}

type cloud struct {
	db *gorm.DB
}

func NewCloud(db *gorm.DB) CloudInterface {
	return &cloud{db}
}

func (s *cloud) Create(ctx context.Context, obj *model.Cloud) (*model.Cloud, error) {
	// TODO: gorm 的 webhook
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now

	if err := s.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *cloud) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := s.db.Model(&model.Cloud{}).
		Where("id = ? and resource_version = ?", uid, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	return nil
}

func (s *cloud) Delete(ctx context.Context, cid int64) error {
	return s.db.
		Where("id = ?", cid).
		Delete(&model.Cloud{}).
		Error
}

func (s *cloud) Get(ctx context.Context, cid int64) (*model.Cloud, error) {
	var c model.Cloud
	if err := s.db.Where("id = ?", cid).First(&c).Error; err != nil {
		return nil, err
	}

	return &c, nil
}

func (s *cloud) List(ctx context.Context) ([]model.Cloud, error) {
	var cs []model.Cloud
	if err := s.db.Find(&cs).Error; err != nil {
		return nil, err
	}

	return cs, nil
}
