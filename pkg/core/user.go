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

	"github.com/caoyingjunz/gopixiu/api/types"
	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/db/model"
	"github.com/caoyingjunz/gopixiu/pkg/log"
)

type UserGetter interface {
	User() UserInterface
}

type UserInterface interface {
	Create(ctx context.Context, obj *types.User) error
	Update(ctx context.Context, obj *types.User) error
	Delete(ctx context.Context, uid int64) error
	Get(ctx context.Context, uid int64) (*types.User, error)
	List(ctx context.Context) ([]types.User, error)

	GetByName(ctx context.Context, name string) (*types.User, error)
	GetJWTKey() string
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

func (u *user) Create(ctx context.Context, obj *types.User) error {
	// TODO 校验参数合法性
	if _, err := u.factory.User().Create(ctx, &model.User{
		Name:        obj.Name,
		Password:    obj.Password,
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

// Update TODO
func (u *user) Update(ctx context.Context, obj *types.User) error {

	return nil
}

func (u *user) Delete(ctx context.Context, uid int64) error {
	return u.factory.User().Delete(ctx, uid)
}

func (u *user) Get(ctx context.Context, uid int64) (*types.User, error) {
	modelUser, err := u.factory.User().Get(ctx, uid)
	if err != nil {
		log.Logger.Errorf("failed to get %d user: %v", uid, err)
		return nil, err
	}

	return model2Type(modelUser), nil
}

func (u *user) List(ctx context.Context) ([]types.User, error) {
	objs, err := u.factory.User().List(ctx)
	if err != nil {
		log.Logger.Errorf("failed to get user list: %v", err)
		return nil, err
	}

	var users []types.User
	for _, obj := range objs {
		users = append(users, *model2Type(&obj))
	}
	return users, nil
}

func (u *user) GetByName(ctx context.Context, name string) (*types.User, error) {
	obj, err := u.factory.User().GetByName(ctx, name)
	if err != nil {
		log.Logger.Errorf("failed to get user by name %s: %v", name, err)
		return nil, err
	}

	return model2Type(obj), nil
}

func (u *user) GetJWTKey() string {
	defaultJWTKey := "46JdPlYNLsNaPqBH"
	jwtKey := u.ComponentConfig.Default.JWTKey
	if len(jwtKey) == 0 {
		return defaultJWTKey
	} else {
		return jwtKey
	}
}

func model2Type(u *model.User) *types.User {
	return &types.User{
		Id:              u.Id,
		ResourceVersion: u.ResourceVersion,
		Name:            u.Name,
		Password:        u.Password,
		Status:          u.Status,
		Role:            u.Role,
		Email:           u.Email,
		Description:     u.Description,
	}
}
