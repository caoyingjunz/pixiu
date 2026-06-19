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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type DistributionInterface interface {
	CreateDistribution(ctx context.Context, object *model.Distribution) (*model.Distribution, error)
	UpdateDistribution(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error
	DeleteDistribution(ctx context.Context, id int64) (*model.Distribution, error)
	GetDistribution(ctx context.Context, id int64) (*model.Distribution, error)
	GetDistributionByName(ctx context.Context, name string) (*model.Distribution, error)
	GetDistributionByFamilyName(ctx context.Context, family, name string) (*model.Distribution, error)
	ListDistributions(ctx context.Context, opts ...Options) ([]model.Distribution, error)
	CountDistributions(ctx context.Context, opts ...Options) (int64, error)
}

type distribution struct {
	db *gorm.DB
}

func newDistribution(db *gorm.DB) DistributionInterface {
	return &distribution{db: db}
}

func (d *distribution) CreateDistribution(ctx context.Context, object *model.Distribution) (*model.Distribution, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := d.db.WithContext(ctx).Create(object).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (d *distribution) UpdateDistribution(ctx context.Context, id int64, resourceVersion int64, updates map[string]interface{}) error {
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := d.db.WithContext(ctx).Model(&model.Distribution{}).
		Where("id = ? and resource_version = ?", id, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}
	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}
	return nil
}

func (d *distribution) DeleteDistribution(ctx context.Context, id int64) (*model.Distribution, error) {
	object, err := d.GetDistribution(ctx, id)
	if err != nil {
		return nil, err
	}
	if object == nil {
		return nil, nil
	}
	if err = d.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Distribution{}).Error; err != nil {
		return nil, err
	}
	return object, nil
}

func (d *distribution) GetDistribution(ctx context.Context, id int64) (*model.Distribution, error) {
	var object model.Distribution
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (d *distribution) GetDistributionByName(ctx context.Context, name string) (*model.Distribution, error) {
	var object model.Distribution
	if err := d.db.WithContext(ctx).Where("name = ?", name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (d *distribution) GetDistributionByFamilyName(ctx context.Context, family, name string) (*model.Distribution, error) {
	var object model.Distribution
	if err := d.db.WithContext(ctx).Where("family = ? and name = ?", family, name).First(&object).Error; err != nil {
		if errors.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &object, nil
}

func (d *distribution) ListDistributions(ctx context.Context, opts ...Options) ([]model.Distribution, error) {
	var objects []model.Distribution
	tx := d.db.WithContext(ctx)
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Order("family asc, name asc").Find(&objects).Error; err != nil {
		return nil, err
	}
	return objects, nil
}

func (d *distribution) CountDistributions(ctx context.Context, opts ...Options) (int64, error) {
	var total int64
	tx := d.db.WithContext(ctx).Model(&model.Distribution{})
	for _, opt := range opts {
		tx = opt(tx)
	}
	if err := tx.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func WithDistributionFamily(family string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if family == "" {
			return tx
		}
		return tx.Where("family = ?", family)
	}
}

func WithDistributionNameLike(name string) Options {
	return func(tx *gorm.DB) *gorm.DB {
		if name == "" {
			return tx
		}
		return tx.Where("name like ?", "%"+name+"%")
	}
}
