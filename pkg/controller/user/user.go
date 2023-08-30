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
	"fmt"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
)

type UserGetter interface {
	User() Interface
}

type Interface interface {
	Create(ctx context.Context, user *types.User) error
	Update(ctx context.Context, userId int64, clu *types.User) error
	Delete(ctx context.Context, userId int64) error
	Get(ctx context.Context, userId int64) (*types.User, error)
	List(ctx context.Context) ([]types.User, error)
}

type user struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

// 创建用户前置检查
// 用户名和密码不能为空
// TODO: 其他检查
func (u *user) preCreate(ctx context.Context, user *types.User) error {
	if len(user.Name) == 0 || len(user.Password) == 0 {
		return fmt.Errorf("user name or password may not be empty")
	}

	// TODO: 对密码进行复杂度校验
	return nil
}

func (u *user) Create(ctx context.Context, user *types.User) error {
	if err := u.preCreate(ctx, user); err != nil {
		return err
	}

	encrypt, err := util.EncryptUserPassword(user.Password)
	if err != nil {
		klog.Errorf("failed to encrypt user password: %v", err)
		return err
	}

	if _, err = u.factory.User().Create(ctx, &model.User{
		Name:        user.Name,
		Password:    encrypt,
		Status:      user.Status,
		Role:        user.Role,
		Email:       user.Email,
		Description: user.Description,
	}); err != nil {
		klog.Errorf("failed to create user %s: %v", user.Name, err)
		return err
	}

	return nil
}

// Update
// TODO: 暂时不做实现
func (u *user) Update(ctx context.Context, userId int64, user *types.User) error {
	return nil
}

func (u *user) Delete(ctx context.Context, userId int64) error {
	if err := u.factory.User().Delete(ctx, userId); err != nil {
		klog.Errorf("failed to delete user(%d): %v", userId, err)
		return err
	}

	return nil
}

func (u *user) Get(ctx context.Context, userId int64) (*types.User, error) {
	object, err := u.factory.User().Get(ctx, userId)
	if err != nil {
		return nil, err
	}

	return model2Type(object), nil
}

func (u *user) List(ctx context.Context) ([]types.User, error) {
	objects, err := u.factory.User().List(ctx)
	if err != nil {
		klog.Errorf("failed to get user list: %v", err)
		return nil, err
	}

	var users []types.User
	for _, object := range objects {
		users = append(users, *model2Type(&object))
	}

	return users, nil
}

// 将 model user 转换成 types
func model2Type(o *model.User) *types.User {
	return &types.User{
		PixiuMeta: types.PixiuMeta{
			Id:              o.Id,
			ResourceVersion: o.ResourceVersion,
		},
		Name:        o.Name,
		Description: o.Description,
		Status:      o.Status,
		Role:        o.Role,
		Email:       o.Email,
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
	}
}

func NewUser(cfg config.Config, f db.ShareDaoFactory) *user {
	return &user{
		cc:      cfg,
		factory: f,
	}
}
