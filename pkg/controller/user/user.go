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

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

type UserGetter interface {
	User() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateUserRequest) error
	Update(ctx context.Context, userId int64, req *types.UpdateUserRequest) error
	UpdatePassword(ctx context.Context, userId int64, req *types.UpdateUserPasswordRequest) error
	Delete(ctx context.Context, userId int64) error
	Get(ctx context.Context, userId int64) (*types.User, error)
	List(ctx context.Context, opts types.ListOptions) ([]types.User, error)

	// GetCount 仅获取用户数量
	GetCount(ctx context.Context, opts types.ListOptions) (int64, error)

	Login(ctx context.Context, req *types.LoginRequest) (string, error)
	Logout(ctx context.Context, userId int64) error
}

type user struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (u *user) Create(ctx context.Context, req *types.CreateUserRequest) error {
	encrypt, err := util.EncryptUserPassword(req.Password)
	if err != nil {
		klog.Errorf("failed to encrypt user password: %v", err)
		return errors.ErrServerInternal
	}

	object, err := u.factory.User().GetUserByName(ctx, req.Name)
	if err != nil {
		klog.Errorf("failed to get user %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}
	if object != nil {
		return errors.ErrUserExists
	}

	if _, err = u.factory.User().Create(ctx, &model.User{
		Name:     req.Name,
		Password: encrypt,
		// Status:      req.Status,
		Role:        req.Role,
		Email:       req.Email,
		Description: req.Description,
	}); err != nil {
		klog.Errorf("failed to create user %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (u *user) Update(ctx context.Context, uid int64, req *types.UpdateUserRequest) error {
	updates := map[string]interface{}{
		"email":       req.Email,
		"description": req.Description,
	}
	if err := u.factory.User().Update(ctx, uid, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update user(%d): %v", uid, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (u *user) UpdatePassword(ctx context.Context, uid int64, req *types.UpdateUserPasswordRequest) error {
	object, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", uid, err)
		return errors.ErrServerInternal
	}
	if object == nil {
		return errors.ErrUserNotFound
	}

	if err = util.ValidateUserPassword(object.Password, req.Old); err != nil {
		klog.Errorf("检验用户密码失败: %v", err)
		return errors.ErrInvalidPassword
	}

	newPass, err := util.EncryptUserPassword(req.New)
	if err != nil {
		klog.Errorf("failed to encrypt user password: %v", err)
		return errors.ErrServerInternal
	}
	if req.New == req.Old {
		return errors.ErrDuplicatedPassword
	}

	updates := map[string]interface{}{
		"password": newPass,
	}
	if err := u.factory.User().Update(ctx, uid, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update user(%d): %v", uid, err)
		return errors.ErrServerInternal
	}
	return nil
}

func (u *user) Delete(ctx context.Context, userId int64) error {
	if err := u.factory.User().Delete(ctx, userId); err != nil {
		klog.Errorf("failed to delete user(%d): %v", userId, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (u *user) Get(ctx context.Context, userId int64) (*types.User, error) {
	object, err := u.factory.User().Get(ctx, userId)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", userId, err)
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrUserNotFound
	}

	return model2Type(object), nil
}

func (u *user) List(ctx context.Context, opts types.ListOptions) ([]types.User, error) {
	objects, err := u.factory.User().List(ctx)
	if err != nil {
		klog.Errorf("failed to get user list: %v", err)
		return nil, errors.ErrServerInternal
	}

	var users []types.User
	for _, object := range objects {
		users = append(users, *model2Type(&object))
	}

	return users, nil
}

func (u *user) GetCount(ctx context.Context, opts types.ListOptions) (int64, error) {
	userCount, err := u.factory.User().Count(ctx)
	if err != nil {
		klog.Errorf("failed to get user counts: %v", err)
		return 0, errors.ErrServerInternal
	}

	return userCount, nil
}

func (u *user) Login(ctx context.Context, req *types.LoginRequest) (string, error) {
	object, err := u.factory.User().GetUserByName(ctx, req.Name)
	if err != nil {
		return "", errors.ErrServerInternal
	}
	if object == nil {
		return "", errors.ErrUserNotFound
	}
	if err = util.ValidateUserPassword(object.Password, req.Password); err != nil {
		klog.Errorf("检验用户密码失败: %v", err)
		return "", errors.ErrInvalidPassword
	}

	// 生成登陆的 token 信息
	key := u.GetTokenKey()
	token, err := tokenutil.GenerateToken(object.Id, object.Name, key)
	if err != nil {
		return "", fmt.Errorf("生成用户 token 失败: %v", err)
	}
	return token, nil
}

// Logout
// TODO
func (u *user) Logout(ctx context.Context, userId int64) error {
	return nil
}

func (u *user) GetTokenKey() []byte {
	k := u.cc.Default.JWTKey
	return []byte(k)

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
		// Status:      o.Status,
		Role:  o.Role,
		Email: o.Email,
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
