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

package db

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/caoyingjunz/pixiu/pkg/db/model"
)

type UserInterface interface {
	Create(ctx context.Context, object *model.User) (*model.User, error)
	Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context) ([]model.User, error)

	Count(ctx context.Context) (int64, error)

	GetUserByName(ctx context.Context, userName string) (*model.User, error)
}

type user struct {
	db *gorm.DB
}

func (u *user) Create(ctx context.Context, object *model.User) (*model.User, error) {
	now := time.Now()
	object.GmtCreate = now
	object.GmtModified = now

	if err := u.db.Create(object).Error; err != nil {
		return nil, err
	}

	return object, nil
}

func (u *user) Update(ctx context.Context, uid int64, resourceVersion int64, updates map[string]interface{}) error {
	return nil
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.db.Where("id = ?", uid).Delete(&model.User{}).Error
}

func (u *user) Get(ctx context.Context, uid int64) (*model.User, error) {
	var object model.User
	if err := u.db.Where("id = ?", uid).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

// List 获取用户列表
// TODO: 暂时不做分页考虑
func (u *user) List(ctx context.Context) ([]model.User, error) {
	var objects []model.User
	if err := u.db.Find(&objects).Error; err != nil {
		return nil, err
	}

	return objects, nil
}

func (u *user) Count(ctx context.Context) (int64, error) {
	var total int64
	if err := u.db.Model(&model.User{}).Count(&total).Error; err != nil {
		return 0, err
	}

	return total, nil
}

func (u *user) GetUserByName(ctx context.Context, userName string) (*model.User, error) {
	var object model.User
	if err := u.db.Where("name = ?", userName).First(&object).Error; err != nil {
		return nil, err
	}

	return &object, nil
}

func newUser(db *gorm.DB) *user {
	return &user{db}
}
