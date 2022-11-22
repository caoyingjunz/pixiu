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

package core

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	pixiumeta "github.com/caoyingjunz/gopixiu/api/meta"
	"github.com/caoyingjunz/gopixiu/api/server/httputils"
	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/errors"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	typesv2 "github.com/caoyingjunz/gopixiu/pkg/types"
)

const defaultJWTKey string = "gopixiu"

type UserGetter interface {
	User() UserInterface
}

type UserInterface interface {
	Create(ctx context.Context, obj *types.User) error
	Update(ctx context.Context, obj *types.User) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*types.User, error)
	// List 分页查询
	List(ctx context.Context, selector *pixiumeta.ListSelector) (interface{}, error)

	Login(ctx context.Context, obj *types.User) (string, error)

	ChangePassword(ctx context.Context, uid int64, obj *types.Password) error // ChangePassword 修改密码
	ResetPassword(ctx context.Context, uid int64, loginId int64) error        // ResetPassword 重置密码

	// GetJWTKey 获取 jwt key
	GetJWTKey() []byte

	GetRoleIDByUser(ctx context.Context, uid int64) (*[]model.Role, error)
	SetUserRoles(ctx context.Context, uid int64, rids []int64) (err error)
	GetButtonsByUserID(ctx context.Context, uid int64) (*[]string, error)
	GetLeftMenusByUserID(ctx context.Context, uid int64) (*[]model.Menu, error)
	UpdateStatus(c context.Context, userId, status int64) error
}

type user struct {
	ComponentConfig config.Config
	app             *pixiu
	factory         db.ShareDaoFactory
}

func newUser(c *pixiu) UserInterface {
	return &user{
		ComponentConfig: c.cfg,
		app:             c,
		factory:         c.factory,
	}
}

// 创建前检查：
// 1. 用户名不能为空
// 2. 用户密码不能为空
// 3. 其他创建前检查
func (u *user) preCreate(ctx context.Context, obj *types.User) error {
	if len(obj.Name) == 0 || len(obj.Password) == 0 {
		return fmt.Errorf("user name or password could not be empty")
	}

	return nil
}

func (u *user) Create(ctx context.Context, obj *types.User) error {
	if err := u.preCreate(ctx, obj); err != nil {
		log.Logger.Errorf("failed to pre-check for created: %v", err)
		return err
	}

	// 对密码进行加密存储
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(obj.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err = u.factory.User().Create(ctx, &model.User{
		Name:        obj.Name,
		Password:    string(encryptedPassword),
		Status:      obj.Status,
		Role:        obj.Role,
		Email:       obj.Email,
		Description: obj.Description,
	}); err != nil {
		log.Logger.Errorf("failed to create user %s: %v", obj.Name, err)
		return err
	}

	return nil
}

func (u *user) Update(ctx context.Context, obj *types.User) error {
	oldUser, err := u.factory.User().Get(ctx, obj.Id)
	if err != nil {
		log.Logger.Errorf("failed to get user %d: %v", obj.Id)
		return err
	}

	updates := u.parseUserUpdates(oldUser, obj)
	if len(updates) == 0 {
		return nil
	}
	if err = u.factory.User().Update(ctx, obj.Id, obj.ResourceVersion, updates); err != nil {
		log.Logger.Errorf("failed to update user %d: %v", obj.Id, err)
		return err
	}

	return nil
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	if err := u.factory.User().Delete(ctx, uid); err != nil {
		log.Logger.Errorf("failed to delete user id %d: %v", uid, err)
		return err
	}

	return nil
}

func (u *user) Get(ctx context.Context, uid int64) (*types.User, error) {
	modelUser, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		log.Logger.Errorf("failed to get %d user: %v", uid, err)
		return nil, err
	}

	return model2Type(modelUser), nil
}

func (u *user) List(ctx context.Context, selector *pixiumeta.ListSelector) (interface{}, error) {
	userObjs, total, err := u.factory.User().List(ctx, selector.Page, selector.Limit)
	if err != nil {
		log.Logger.Errorf("failed to list page %d size %d usrs: %v", selector.Page, selector.Limit, err)
		return nil, err
	}
	var us []types.User
	for _, userObj := range userObjs {
		us = append(us, *model2Type(&userObj))
	}

	return map[string]interface{}{
		"users": us,
		"total": total,
	}, nil
}

func (u *user) preLogin(ctx context.Context, obj *types.User) error {
	if len(obj.Name) == 0 {
		return fmt.Errorf("invalid empty user name")
	}
	if len(obj.Password) == 0 {
		return fmt.Errorf("invalid empty user password")
	}

	return nil
}

func (u *user) Login(ctx context.Context, obj *types.User) (string, error) {
	if err := u.preLogin(ctx, obj); err != nil {
		log.Logger.Errorf("failed to pre-check for login: %v", err)
		return "", err
	}

	userObj, err := u.factory.User().GetByName(context.TODO(), obj.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", errors.ErrUserNotFound
		}
		return "", err
	}
	// To ensure login password is correct
	if err = bcrypt.CompareHashAndPassword([]byte(userObj.Password), []byte(obj.Password)); err != nil {
		return "", errors.ErrUserPassword
	}
	// TODO: 根据用户的登陆状态

	// 生成 token，并返回
	return httputils.GenerateToken(userObj.Id, obj.Name, u.GetJWTKey())
}

// 修改密码前的参数校验
func (u *user) preChangePassword(ctx context.Context, uid int64, obj *types.Password) error {
	// 1. 新旧密码一样
	if obj.OriginPassword == obj.Password {
		return fmt.Errorf("the origin password is equal to the password")
	}

	// 2. 两次输入的密码不一致
	if obj.Password != obj.ConfirmPassword {
		return fmt.Errorf("the confrim password is equal to the password")
	}

	// 3. 请求参数中的 uid 和 token 中的 uid 不一致
	// - 普通用户禁止修改他人的密码
	// TODO - 管理员可以修改他人的密码
	if uid != obj.UserId {
		return fmt.Errorf("cannot change other user's (%d) password", obj.UserId)
	}

	return nil
}

func (u *user) ChangePassword(ctx context.Context, uid int64, obj *types.Password) error {
	if err := u.preChangePassword(ctx, uid, obj); err != nil {
		log.Logger.Errorf("failed to change password: %v", err)
		return err
	}

	// 获取当前密码，检查原始密码是否正确
	userObj, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		log.Logger.Errorf("failed to get user by id %d: %v", uid, err)
		return err
	}
	// To ensure origin password is correct
	if err = bcrypt.CompareHashAndPassword([]byte(userObj.Password), []byte(obj.OriginPassword)); err != nil {
		return fmt.Errorf("incorrect origin password")
	}

	// 密码加密存储
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(obj.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Logger.Errorf("failed to encrypted %d password: %v", uid, err)
		return err
	}
	// TODO: 未对 ResourceVersion 进行校验，修改密码是非高频操作，暂时不做校验
	if err = u.factory.User().Update(ctx, uid, userObj.ResourceVersion, map[string]interface{}{"password": encryptedPassword}); err != nil {
		log.Logger.Errorf("failed to change %d password %d: %v", uid, err)
		return err
	}

	return nil
}

func (u *user) ResetPassword(ctx context.Context, uid int64, loginId int64) error {
	// 获取当前登陆用户
	loginObj, err := u.factory.User().Get(ctx, loginId)
	if err != nil {
		log.Logger.Errorf("failed to get admin by id %d: %v", loginId, err)
		return err
	}
	// 管理员角色可以重置密码
	if typesv2.AdminRole != loginObj.Role {
		return fmt.Errorf("only admin can reset password")
	}

	// 密码加密存储
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(typesv2.DefaultPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Logger.Errorf("failed to encrypted %d password: %v", uid, err)
		return err
	}
	if err = u.factory.User().UpdateInternal(ctx, uid, map[string]interface{}{"password": encryptedPassword}); err != nil {
		log.Logger.Errorf("failed to reset %d password %d: %v", uid, err)
		return err
	}

	return nil
}
func (u *user) GetJWTKey() []byte {
	jwtKey := u.ComponentConfig.Default.JWTKey
	if len(jwtKey) == 0 {
		jwtKey = defaultJWTKey
	}

	return []byte(jwtKey)
}

func model2Type(u *model.User) *types.User {
	return &types.User{
		Id:              u.Id,
		ResourceVersion: u.ResourceVersion,
		Name:            u.Name,
		Status:          u.Status,
		Role:            u.Role,
		Email:           u.Email,
		Description:     u.Description,
		TimeOption:      types.FormatTime(u.GmtCreate, u.GmtModified),
	}
}

// TODO: 后续调整为全量更新
func (u *user) parseUserUpdates(oldObj *model.User, newObj *types.User) map[string]interface{} {
	updates := make(map[string]interface{})

	if oldObj.Status != newObj.Status { // 更新状态
		updates["status"] = newObj.Status
	}
	if oldObj.Role != newObj.Role { // 更新用户角色
		updates["role"] = newObj.Role
	}
	if oldObj.Email != newObj.Email { // 更新邮件
		updates["email"] = newObj.Email
	}
	if oldObj.Description != newObj.Description { // 更新描述
		updates["description"] = newObj.Description
	}

	return updates
}

func (u *user) GetRoleIDByUser(ctx context.Context, uid int64) (*[]model.Role, error) {
	roleInfo, err := u.factory.User().GetRoleIDByUser(ctx, uid)
	if err != nil {
		log.Logger.Error(err)
	}
	return roleInfo, err
}

func (u *user) SetUserRoles(ctx context.Context, uid int64, rids []int64) (err error) {
	// 添加规则到rules表
	err = u.factory.Authentication().AddRoleForUser(ctx, uid, rids)
	if err != nil {
		log.Logger.Error(err)
		return err
	}

	// 配置role_users表
	err = u.factory.User().SetUserRoles(ctx, uid, rids)
	if err != nil { // 如果失败,则清除rules已添加的规则
		log.Logger.Error(err)
		for _, roleId := range rids {
			err = u.factory.Authentication().DeleteRoleWithUser(ctx, uid, roleId)
			if err != nil {
				log.Logger.Error(err)
				break
			}
		}
		return
	}

	return
}

func (u *user) GetButtonsByUserID(ctx context.Context, uid int64) (*[]string, error) {
	res, err := u.factory.User().GetButtonsByUserID(ctx, uid)
	if err != nil {
		log.Logger.Error(err)
		return nil, err
	}
	var menus []string
	for _, v := range *res {
		menus = append(menus, v.Code)
	}

	return &menus, nil
}

func (u *user) GetLeftMenusByUserID(ctx context.Context, uid int64) (menus *[]model.Menu, err error) {
	menus, err = u.factory.User().GetLeftMenusByUserID(ctx, uid)
	if err != nil {
		log.Logger.Error(err)
	}
	return
}

func (u *user) UpdateStatus(c context.Context, userId, status int64) error {
	return u.factory.User().UpdateStatus(c, userId, status)
}
