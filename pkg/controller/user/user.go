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

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/types"
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

func (u *user) Create(ctx context.Context, user *types.User) error {
	return nil
}

func (u *user) Update(ctx context.Context, userId int64, user *types.User) error {
	return nil
}

func (u *user) Delete(ctx context.Context, userId int64) error {
	return nil
}

func (u *user) Get(ctx context.Context, userId int64) (*types.User, error) {
	return nil, nil
}

func (u *user) List(ctx context.Context) ([]types.User, error) {
	return nil, nil
}

func NewUser(cfg config.Config, f db.ShareDaoFactory) *user {
	return &user{
		cc:      cfg,
		factory: f,
	}
}
