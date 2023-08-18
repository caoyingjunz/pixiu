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

package user

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/util/errors"
)

type Interface interface {
	Create(ctx context.Context, obj *model.User) (*model.User, error)
	Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context, page int, pageSize int) ([]model.User, int64, error)
}

type user struct {
	db *gorm.DB
}

func NewUser(db *gorm.DB) Interface {
	return &user{db}
}

func (u *user) Create(ctx context.Context, obj *model.User) (*model.User, error) {
	// 系统维护字段
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now

	if err := u.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (u *user) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	// 系统维护字段
	updates["gmt_modified"] = time.Now()
	updates["resource_version"] = resourceVersion + 1

	f := u.db.Model(&model.User{}).
		Where("id = ? and resource_version = ?", uid, resourceVersion).
		Updates(updates)
	if f.Error != nil {
		return f.Error
	}

	if f.RowsAffected == 0 {
		return errors.ErrRecordNotUpdate
	}

	return nil
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.db.
		Where("id = ?", uid).
		Delete(&model.User{}).
		Error
}

func (u *user) Get(ctx context.Context, uid int64) (*model.User, error) {
	var obj model.User
	if err := u.db.Where("id = ?", uid).First(&obj).Error; err != nil {
		return nil, err
	}

	return &obj, nil
}

// List 分页查询
func (u *user) List(ctx context.Context, page, pageSize int) ([]model.User, int64, error) {
	var (
		us    []model.User
		total int64
	)
	if err := u.db.Limit(pageSize).Offset((page - 1) * pageSize).Find(&us).Error; err != nil {
		return nil, 0, err
	}
	if err := u.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	return us, total, nil
}
