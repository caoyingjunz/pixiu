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

	"github.com/caoyingjunz/gopixiu/pkg/db/model"
)

type UserInterface interface {
	Create(ctx context.Context, obj *model.User) (*model.User, error)
	Update(ctx context.Context, obj *model.User) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*model.User, error)
	List(ctx context.Context) ([]model.User, error)

	GetByName(ctx context.Context, name string) (*model.User, error)
}

type user struct {
	db *gorm.DB
}

func NewUser(db *gorm.DB) UserInterface {
	return &user{db}
}

func (u *user) Create(ctx context.Context, obj *model.User) (*model.User, error) {
	now := time.Now()
	obj.GmtCreate = now
	obj.GmtModified = now

	if err := u.db.Create(obj).Error; err != nil {
		return nil, err
	}
	return obj, nil
}

func (u *user) Update(ctx context.Context, modelUser *model.User) error {
	return u.db.Updates(*modelUser).Error
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.db.Delete(model.User{}, uid).Error
}

func (u *user) Get(ctx context.Context, uid int64) (*model.User, error) {
	var modelUser model.User
	if err := u.db.Where("id = ?", uid).First(&modelUser).Error; err != nil {
		return nil, err
	}
	return &modelUser, nil
}

func (u *user) List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if tx := u.db.Find(&users); tx.Error != nil {
		return nil, tx.Error
	}
	return users, nil
}

func (u *user) GetByName(ctx context.Context, name string) (*model.User, error) {
	var modelUser model.User
	if err := u.db.Where("name = ?", name).First(&modelUser).Error; err != nil {
		return nil, err
	}
	return &modelUser, nil
}
