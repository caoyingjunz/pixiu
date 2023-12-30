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

package tenant

import (
	"context"

	"github.com/caoyingjunz/pixiu/cmd/app/config"
	"github.com/caoyingjunz/pixiu/pkg/db"
	"github.com/caoyingjunz/pixiu/pkg/types"
)

type TenantGetter interface {
	Tenant() Interface
}

type Interface interface {
	Create(ctx context.Context, object *types.Tenant) error
	Update(ctx context.Context, tid int64, object *types.Tenant) error
	Delete(ctx context.Context, tid int64) error
	Get(ctx context.Context, tid int64) (*types.Tenant, error)
	List(ctx context.Context) ([]types.Tenant, error)
}

type tenant struct {
	cc      config.Config
	factory db.ShareDaoFactory
}

func (t *tenant) Create(ctx context.Context, object *types.Tenant) error {
	return nil
}

func (t *tenant) Update(ctx context.Context, tid int64, object *types.Tenant) error {
	return nil
}

func (t *tenant) Delete(ctx context.Context, tid int64) error {
	return nil
}

func (t *tenant) Get(ctx context.Context, tid int64) (*types.Tenant, error) {
	return nil, nil
}

func (t *tenant) List(ctx context.Context) ([]types.Tenant, error) {
	return nil, nil
}

func NewTenant(cfg config.Config, f db.ShareDaoFactory) *tenant {
	return &tenant{
		cc:      cfg,
		factory: f,
	}
}
