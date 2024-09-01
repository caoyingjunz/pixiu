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

	"github.com/casbin/casbin/v2"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/api/server/errors"
	"github.com/caoyingjunz/pixiu/api/server/httputils"
	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/client"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/db/model"
	"github.com/caoyingjunz/pixiu/pkg/types"
	"github.com/caoyingjunz/pixiu/pkg/util"
	tokenutil "github.com/caoyingjunz/pixiu/pkg/util/token"
)

var (
	userIndexer  client.UserCache
	tokenIndexer client.TokenCache
)

func init() {
	userIndexer = *client.NewUserCache()
	tokenIndexer = *client.NewTokenCache()
}

type UserGetter interface {
	User() Interface
}

type Interface interface {
	Create(ctx context.Context, req *types.CreateUserRequest) error
	Update(ctx context.Context, userId int64, req *types.UpdateUserRequest) error
	Delete(ctx context.Context, userId int64) error
	Get(ctx context.Context, userId int64) (*types.User, error)
	List(ctx context.Context, opts types.ListOptions) ([]types.User, error)

	// UpdatePassword 用户修改密码或者管理员重置密码
	UpdatePassword(ctx context.Context, userId int64, req *types.UpdateUserPasswordRequest) error
	// GetCount 仅获取用户数量
	GetCount(ctx context.Context, opts types.ListOptions) (int64, error)
	// GetStatus 获取用户状态，优先从缓存获取，如果没有则从库里获取，然后同步到缓存
	GetStatus(ctx context.Context, uid int64) (int, error)

	Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error)
	Logout(ctx context.Context, userId int64) error
	GetLoginToken(ctx context.Context, userId int64) (string, error)
}

type user struct {
	cc       config.Config
	factory  db.ShareDaoFactory
	enforcer *casbin.SyncedEnforcer
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
		err = errors.ErrUserExists // 记录错误
		return err
	}

	txFunc := func() (err error) {
		if req.Role == model.RoleRoot {
			bindings := model.NewGroupBinding(req.Name, model.AdminGroup)
			_, err = u.enforcer.AddGroupingPolicy(bindings.Raw())
		}
		return
	}

	if _, err = u.factory.User().Create(ctx, &model.User{
		Name:        req.Name,
		Password:    encrypt,
		Status:      req.Status,
		Role:        req.Role,
		Email:       req.Email,
		Description: req.Description,
	}, txFunc); err != nil {
		klog.Errorf("failed to create user %s: %v", req.Name, err)
		return errors.ErrServerInternal
	}

	return nil
}

func (u *user) Update(ctx context.Context, uid int64, req *types.UpdateUserRequest) error {
	updates := map[string]interface{}{
		"status":      req.Status,
		"email":       req.Email,
		"description": req.Description,
	}
	if err := u.factory.User().Update(ctx, uid, *req.ResourceVersion, updates); err != nil {
		klog.Errorf("failed to update user(%d): %v", uid, err)
		return errors.ErrServerInternal
	}

	userIndexer.Set(uid, int(req.Status))
	return nil
}

func (u *user) preResetPassword(ctx context.Context, userId int64, operatorId int64, req *types.UpdateUserPasswordRequest) error {
	// 操作人必须具备管理员权限
	operator, err := u.Get(ctx, operatorId)
	if err != nil {
		return err
	}

	if operator.Role != model.RoleRoot {
		return fmt.Errorf("非超级管理员，不允许重置用户密码")
	}
	return nil
}

func (u *user) preChangePassword(ctx context.Context, userId int64, operatorId int64, req *types.UpdateUserPasswordRequest) error {
	if operatorId != userId {
		return fmt.Errorf("用户只能修改自己的密码")
	}
	object, err := u.Get(ctx, userId)
	if err != nil {
		return err
	}

	// 校验旧密码是否正确
	if err = util.ValidateUserPassword(object.Password, req.Old); err != nil {
		klog.Errorf("检验用户密码失败: %v", err)
		return errors.ErrInvalidPassword
	}
	return nil
}

// UpdatePassword 支持用户修改密码和管理员重置密码
// 修改密码: 用户只能修改自己密码
// 重启密码: 管理员可以重置他人密码
func (u *user) UpdatePassword(ctx context.Context, userId int64, req *types.UpdateUserPasswordRequest) error {
	// 新老密码不允许相同
	if req.New == req.Old {
		return errors.ErrDuplicatedPassword
	}

	operatorId, err := httputils.GetUserIdFromContext(ctx)
	if err != nil {
		return err
	}

	if req.Reset {
		// 管理员重置密码前置检查
		if err = u.preResetPassword(ctx, userId, operatorId, req); err != nil {
			return err
		}
	} else {
		// 用户修改密码前置检查
		if err = u.preChangePassword(ctx, userId, operatorId, req); err != nil {
			return err
		}
	}

	newPass, err := util.EncryptUserPassword(req.New)
	if err != nil {
		klog.Errorf("failed to encrypt user password: %v", err)
		return errors.ErrServerInternal
	}
	if err = u.factory.User().Update(ctx, userId, *req.ResourceVersion, map[string]interface{}{
		"password": newPass,
	}); err != nil {
		klog.Errorf("failed to update user(%d) password: %v", userId, err)
		return errors.ErrServerInternal
	}

	tokenIndexer.Delete(userId)
	return nil
}

func (u *user) Delete(ctx context.Context, userId int64) error {
	if err := u.factory.User().Delete(ctx, userId); err != nil {
		klog.Errorf("failed to delete user(%d): %v", userId, err)
		return errors.ErrServerInternal
	}

	userIndexer.Delete(userId)
	tokenIndexer.Delete(userId)
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

// GetStatus 获取用户状态，优先从缓存获取，如果没有则从库里获取，然后同步到缓存
func (u *user) GetStatus(ctx context.Context, uid int64) (int, error) {
	status, ok := userIndexer.Get(uid)
	if ok {
		return status, nil
	}

	object, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		klog.Errorf("failed to get user(%d): %v", uid, err)
		return 0, errors.ErrServerInternal
	}
	if object == nil {
		return 0, errors.ErrUserNotFound
	}

	userIndexer.Set(uid, int(object.Status))
	return int(object.Status), nil
}

func (u *user) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	object, err := u.factory.User().GetUserByName(ctx, req.Name)
	if err != nil {
		return nil, errors.ErrServerInternal
	}
	if object == nil {
		return nil, errors.ErrUserNotFound
	}

	// 如果用户已被禁用，则不允许登陆
	if object.Status == 2 {
		return nil, fmt.Errorf("用户已被禁用")
	}
	if err = util.ValidateUserPassword(object.Password, req.Password); err != nil {
		klog.Errorf("检验用户密码失败: %v", err)
		return nil, errors.ErrInvalidPassword
	}

	// 生成登陆的 token 信息
	key := u.GetTokenKey()
	token, err := tokenutil.GenerateToken(object.Id, object.Name, key)
	if err != nil {
		return nil, fmt.Errorf("生成用户 token 失败: %v", err)
	}

	tokenIndexer.Set(object.Id, token)
	return &types.LoginResponse{
		UserId:   object.Id,
		UserName: object.Name,
		Token:    token,
		Role:     object.Role,
		User:     object,
	}, nil
}

// Logout
// 允许用户登出登陆状态
// TODO: 临时实现，后续优化
func (u *user) Logout(ctx context.Context, userId int64) error {
	tokenIndexer.Delete(userId)
	return nil
}

func (u *user) GetLoginToken(ctx context.Context, userId int64) (string, error) {
	t, exists := tokenIndexer.Get(userId)
	if !exists {
		return "", fmt.Errorf("invalid empty token")
	}

	return t, nil
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
		Status:      o.Status,
		Role:        o.Role,
		Email:       o.Email,
		TimeMeta: types.TimeMeta{
			GmtCreate:   o.GmtCreate,
			GmtModified: o.GmtModified,
		},
	}
}

func NewUser(cfg config.Config, f db.ShareDaoFactory, e *casbin.SyncedEnforcer) *user {
	return &user{
		cc:       cfg,
		factory:  f,
		enforcer: e,
	}
}
